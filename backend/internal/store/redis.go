package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func NewRedis(addr, password string, db int) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Redis{client: client}, nil
}

// Raw 暴露底层客户端，供 SSE 等模块做 pub/sub。
func (r *Redis) Raw() *redis.Client {
	return r.client
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *Redis) SetCache(ctx context.Context, key string, value []byte) error {
	return r.client.Set(ctx, key, value, 0).Err()
}

func (r *Redis) GetCache(ctx context.Context, key string) ([]byte, error) {
	b, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return b, err
}

func (r *Redis) SetCacheWithTTL(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *Redis) GetString(ctx context.Context, key string) (string, error) {
	v, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return v, err
}

func (r *Redis) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *Redis) SetRefreshToken(ctx context.Context, jti, userID string, ttl time.Duration) error {
	return r.client.Set(ctx, "refresh:"+jti, userID, ttl).Err()
}

func (r *Redis) GetRefreshToken(ctx context.Context, jti string) (string, error) {
	v, err := r.client.Get(ctx, "refresh:"+jti).Result()
	if err == redis.Nil {
		return "", nil
	}
	return v, err
}

func (r *Redis) RevokeRefreshToken(ctx context.Context, jti string) error {
	return r.client.Del(ctx, "refresh:"+jti).Err()
}

func (r *Redis) RotateRefreshToken(ctx context.Context, oldJti, newJti, userID string, ttl time.Duration) error {
	pipe := r.client.TxPipeline()
	pipe.Del(ctx, "refresh:"+oldJti)
	pipe.Set(ctx, "refresh:"+newJti, userID, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *Redis) RevokeAllUserTokens(ctx context.Context, userID string, scanCount int64) error {
	iter := r.client.Scan(ctx, 0, "refresh:*", scanCount).Iterator()
	pipe := r.client.TxPipeline()
	for iter.Next(ctx) {
		key := iter.Val()
		val, err := r.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		if val == userID {
			pipe.Del(ctx, key)
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *Redis) IncrementFailure(ctx context.Context, userID string, ttl time.Duration) (int64, error) {
	key := "failcount:" + userID
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		r.client.Expire(ctx, key, ttl)
	}
	return count, nil
}

func (r *Redis) ClearFailure(ctx context.Context, userID string) error {
	return r.client.Del(ctx, "failcount:"+userID).Err()
}

func (r *Redis) GetFailureCount(ctx context.Context, userID string) (int64, error) {
	v, err := r.client.Get(ctx, "failcount:"+userID).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return v, err
}

// lockedSetKey 维护一个被锁账号的集合，使锁定视图可以枚举当前被锁账号，
// 而不必扫描所有用户。账号自然解锁（TTL 过期）后仍留在集合里，
// 由 ListLockedMembers 的调用方用 IsLocked 二次过滤，保证视图与实时锁定态一致。
const lockedSetKey = "auth:locked:set"

func (r *Redis) SetLocked(ctx context.Context, userID string, ttl time.Duration) error {
	pipe := r.client.TxPipeline()
	pipe.Set(ctx, "locked:"+userID, "1", ttl)
	pipe.SAdd(ctx, lockedSetKey, userID)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *Redis) IsLocked(ctx context.Context, userID string) (bool, error) {
	v, err := r.client.Exists(ctx, "locked:"+userID).Result()
	return v > 0, err
}

func (r *Redis) UnlockUser(ctx context.Context, userID string) error {
	pipe := r.client.TxPipeline()
	pipe.Del(ctx, "locked:"+userID)
	pipe.Del(ctx, "failcount:"+userID)
	pipe.SRem(ctx, lockedSetKey, userID)
	_, err := pipe.Exec(ctx)
	return err
}

// ListLockedMembers 返回曾被锁定的账号集合（可能含已自然解锁的残留项），
// 调用方需结合 IsLocked 过滤出真正处于锁定态的账号。
func (r *Redis) ListLockedMembers(ctx context.Context) ([]string, error) {
	return r.client.SMembers(ctx, lockedSetKey).Result()
}

func (r *Redis) LockTTL(ctx context.Context, userID string) (time.Duration, error) {
	return r.client.TTL(ctx, "locked:"+userID).Result()
}

func (r *Redis) SetResetToken(ctx context.Context, token, userID string, ttl time.Duration) error {
	return r.client.Set(ctx, "reset:"+token, userID, ttl).Err()
}

func (r *Redis) ConsumeResetToken(ctx context.Context, token string) (string, error) {
	key := "reset:" + token
	pipe := r.client.TxPipeline()
	get := pipe.Get(ctx, key)
	pipe.Del(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", err
	}
	return get.Val(), nil
}

func (r *Redis) SetIfNotExists(ctx context.Context, key, value string, ttl time.Duration) error {
	ok, err := r.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrConflict
	}
	return nil
}

// IncrUsage 累加用户的 token 用量，prompt/completion 分别用独立 key 计数。
// 调用方为 Node agent 服务，在每轮 LLM 调用后上报。
func (r *Redis) IncrUsage(ctx context.Context, userID string, promptTokens, completionTokens int64) error {
	if promptTokens > 0 {
		if err := r.client.IncrBy(ctx, r.usageKey(userID, "prompt"), promptTokens).Err(); err != nil {
			return err
		}
	}
	if completionTokens > 0 {
		if err := r.client.IncrBy(ctx, r.usageKey(userID, "completion"), completionTokens).Err(); err != nil {
			return err
		}
	}
	return nil
}

// GetUsage 返回用户累计的 prompt/completion token 用量。
func (r *Redis) GetUsage(ctx context.Context, userID string) (prompt, completion int64, err error) {
	p, e1 := r.client.Get(ctx, r.usageKey(userID, "prompt")).Int64()
	if e1 != nil && e1 != redis.Nil {
		return 0, 0, e1
	}
	c, e2 := r.client.Get(ctx, r.usageKey(userID, "completion")).Int64()
	if e2 != nil && e2 != redis.Nil {
		return 0, 0, e2
	}
	return p, c, nil
}

func (r *Redis) usageKey(userID, kind string) string {
	return fmt.Sprintf("agent:usage:%s:%s", userID, kind)
}
