package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Event 是广播给 SSE 客户端的笔记事件。
// Seq 为单调递增序列号（Redis INCR 生成），客户端据此去重与断线重连回放。
// SourceSess 为发起变更的 SSE 会话 ID，扇出时跳过该会话以避免回环。
type Event struct {
	NoteID     string `json:"noteId"`
	Type       string `json:"type"`
	TS         string `json:"ts"`
	SourceSess string `json:"sourceSessionId,omitempty"`
	Seq        int64  `json:"seq"`
	EventID    string `json:"eventId"`
}

func (e Event) String() string {
	b, _ := json.Marshal(e)
	return string(b)
}

const (
	replayCap    = 100
	replayTTL    = 60 * time.Second
	heartbeatInt = 15 * time.Second
	sendBuffer   = 32
)

// Broker 基于 Redis pub/sub 的事件广播器。
//
// 关键设计：每个用户只维护一个 Redis 订阅（userHub），由单个 goroutine
// 将消息扇出到该用户的所有内存会话。旧实现为每个会话各开一个订阅，
// 导致 N 个会话时每条事件被重复投递 N-1 次，已修复。
type Broker struct {
	rdb  *redis.Client
	mu   sync.RWMutex // 保护 hubs 及各 hub.sessions
	hubs map[string]*userHub
}

// userHub 代表一个用户的事件分发单元：一个 Redis 订阅 + 该用户的全部会话。
type userHub struct {
	userID   string
	sessions map[string]chan Event // sessionID -> 接收通道
	cancel   context.CancelFunc
	done     chan struct{}
}

func NewBroker(rdb *redis.Client) *Broker {
	return &Broker{rdb: rdb, hubs: map[string]*userHub{}}
}

// Register 为 (userID, sessionID) 建立广播接收通道。
// 同一用户共享一个 userHub / Redis 订阅；返回接收通道与停止函数。
// 停止函数在会话断开时注销；当用户最后一个会话退出后回收 hub 与订阅。
func (b *Broker) Register(userID, sessionID string) (<-chan Event, func()) {
	out := make(chan Event, sendBuffer)

	b.mu.Lock()
	hub, ok := b.hubs[userID]
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		hub = &userHub{
			userID:   userID,
			sessions: map[string]chan Event{},
			cancel:   cancel,
			done:     make(chan struct{}),
		}
		b.hubs[userID] = hub
		go b.runHub(hub, ctx)
	}
	hub.sessions[sessionID] = out
	b.mu.Unlock()

	stop := func() {
		b.mu.Lock()
		delete(hub.sessions, sessionID)
		empty := len(hub.sessions) == 0
		if empty {
			if cur, ok := b.hubs[userID]; ok && cur == hub {
				delete(b.hubs, userID)
				hub.cancel()
			} else {
				empty = false
			}
		}
		b.mu.Unlock()
		if empty {
			<-hub.done
		}
	}
	return out, stop
}

// runHub 运行单个用户的 Redis 订阅循环，将 pub/sub 消息扇出到所有会话。
func (b *Broker) runHub(hub *userHub, ctx context.Context) {
	defer close(hub.done)
	sub := b.rdb.Subscribe(ctx, channel(hub.userID))
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var ev Event
			if json.Unmarshal([]byte(msg.Payload), &ev) != nil {
				continue
			}
			b.fanout(hub, ev)
		}
	}
}

// fanout 将事件投递给该用户除来源会话外的所有会话。
func (b *Broker) fanout(hub *userHub, ev Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for id, ch := range hub.sessions {
		if id == ev.SourceSess {
			continue
		}
		select {
		case ch <- ev:
		default:
		}
	}
}

// Publish 发布事件到用户 channel 并写入回放列表（最多 100 条，60s TTL）。
// 通过 Redis INCR 分配单调序列号；SourceSess 仅在调用方显式提供时设置，
// 不再生成随机值（旧实现生成的随机 UUID 永不匹配任何会话，过滤失效）。
func (b *Broker) Publish(userID string, ev Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	seq, err := b.rdb.Incr(ctx, seqKey(userID)).Result()
	if err != nil {
		return err
	}
	ev.Seq = seq
	ev.EventID = strconv.FormatInt(seq, 10)
	if ev.TS == "" {
		ev.TS = time.Now().UTC().Format(time.RFC3339)
	}
	payload := ev.String()
	rk := replayKey(userID)
	if err := b.rdb.LPush(ctx, rk, payload).Err(); err != nil {
		return err
	}
	if err := b.rdb.LTrim(ctx, rk, 0, replayCap-1).Err(); err != nil {
		return err
	}
	if err := b.rdb.Expire(ctx, rk, replayTTL).Err(); err != nil {
		return err
	}
	return b.rdb.Publish(ctx, channel(userID), payload).Err()
}

// Replay 返回回放窗口中序列号大于 sinceSeq 的事件（按时间正序）。
// sinceSeq <= 0 时返回空（首次连接无需回放）。
func (b *Broker) Replay(userID string, sinceSeq int64) []Event {
	if sinceSeq <= 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := b.rdb.LRange(ctx, replayKey(userID), 0, -1).Result()
	if err != nil {
		return nil
	}
	var out []Event
	// LPush 使最新在前，倒序遍历恢复时间正序。
	for i := len(raw) - 1; i >= 0; i-- {
		var ev Event
		if json.Unmarshal([]byte(raw[i]), &ev) != nil {
			continue
		}
		if ev.Seq > sinceSeq {
			out = append(out, ev)
		}
	}
	return out
}

func NewHeartbeatTicker() *time.Ticker { return time.NewTicker(heartbeatInt) }

// Heartbeat 返回 SSE 心跳间隔。
func Heartbeat() time.Duration { return heartbeatInt }

func channel(userID string) string   { return fmt.Sprintf("notes:events:%s", userID) }
func replayKey(userID string) string { return fmt.Sprintf("notes:replay:%s", userID) }
func seqKey(userID string) string    { return fmt.Sprintf("notes:seq:%s", userID) }
