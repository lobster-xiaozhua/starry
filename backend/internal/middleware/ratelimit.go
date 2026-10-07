package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"starry/backend/internal/logx"
)

// Counter 是限流计数存储的抽象。
//
// 单实例部署可用进程内计数（memoryCounter）；多实例/容器水平扩容时必须换成共享
// 计数（见 store.NewRedisCounter），否则每个实例各自计数，限流额度会被实例数放大。
type Counter interface {
	// Incr 对 key 计数并返回窗口内的累计次数；key 的过期由实现保证。
	Incr(ctx context.Context, key string, window time.Duration) (int64, error)
}

// Limiter 按“作用域”生产限流中间件：同一作用域共享额度，不同作用域互不影响。
// 通过组合根注入具体实现，业务模块只声明“这个端点限多少”，不关心底层存储。
type Limiter struct {
	counter Counter
}

// NewLimiter 构建限流器。counter 为 nil 时退化为进程内计数。
func NewLimiter(c Counter) *Limiter {
	if c == nil {
		c = NewMemoryCounter()
	}
	return &Limiter{counter: c}
}

// Limit 返回固定窗口限流中间件：窗口内放行 max 次，第 max+1 次返回 429。
//
// 计数失败（例如 Redis 抖动）时选择 fail-open：放行请求并记 warn。
// 限流属于保护性措施，因计数不可用而拒绝全部流量会放大故障；真正的暴力破解
// 由登录失败锁定（failcount/locked）兜底。
func (l *Limiter) Limit(scope string, max int, window time.Duration) gin.HandlerFunc {
	if l == nil || max <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		key := scope + ":" + c.ClientIP()
		count, err := l.counter.Incr(c.Request.Context(), key, window)
		if err != nil {
			logx.L().Warn("rate limit counter unavailable, fail-open",
				slog.String("scope", scope),
				slog.String("request_id", RequestIDFrom(c)),
				slog.String("error", err.Error()))
			c.Next()
			return
		}
		if count > int64(max) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    1029,
				"message": "请求过于频繁，请稍后再试",
				"data":    nil,
			})
			return
		}
		c.Next()
	}
}

// RateLimit 是无状态便利入口：直接以进程内计数限流，适用于本地开发或单实例部署。
// 多实例场景请在组合根构造 Limiter 并通过 Deps 注入。
func RateLimit(max int, window time.Duration) gin.HandlerFunc {
	return NewLimiter(NewMemoryCounter()).Limit("default", max, window)
}

// ---- 进程内计数实现 ----

type memoryCounter struct {
	mu      sync.Mutex
	buckets map[string]windowBucket
}

type windowBucket struct {
	expireAt time.Time
	count    int64
}

// NewMemoryCounter 返回进程内固定窗口计数器。所有实例共用一个后台清扫协程，
// 避免每个中间件都起一个 goroutine。
func NewMemoryCounter() *memoryCounter {
	c := &memoryCounter{buckets: map[string]windowBucket{}}
	registerSweeper(c)
	return c
}

func (c *memoryCounter) Incr(_ context.Context, key string, window time.Duration) (int64, error) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	b, ok := c.buckets[key]
	if !ok || now.After(b.expireAt) {
		b = windowBucket{expireAt: now.Add(window), count: 0}
	}
	b.count++
	c.buckets[key] = b
	return b.count, nil
}

func (c *memoryCounter) sweep(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, b := range c.buckets {
		if now.After(b.expireAt) {
			delete(c.buckets, k)
		}
	}
}

var (
	sweepMu     sync.Mutex
	swept       []*memoryCounter
	sweepOnce   sync.Once
	sweepWindow = 30 * time.Second
)

func registerSweeper(c *memoryCounter) {
	sweepMu.Lock()
	swept = append(swept, c)
	sweepMu.Unlock()
	sweepOnce.Do(func() {
		go func() {
			for range time.Tick(sweepWindow) {
				now := time.Now()
				sweepMu.Lock()
				list := swept
				sweepMu.Unlock()
				for _, c := range list {
					c.sweep(now)
				}
			}
		}()
	})
}
