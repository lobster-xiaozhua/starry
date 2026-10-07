package store

import (
	"context"
	"time"
)

// RedisCounter 是基于 Redis 的共享限流计数器，满足 middleware.Counter 接口
// （鸭子类型，本包不 import middleware，避免基础设施层互相耦合）。
//
// 多实例部署时必须使用它：进程内计数只在单个副本内生效，副本数为 N 时实际额度
// 会变成 N 倍，防刷形同虚设。
type RedisCounter struct {
	r *Redis
}

func NewRedisCounter(r *Redis) *RedisCounter {
	return &RedisCounter{r: r}
}

// Incr 对 key 自增，并在首次写入时设置过期时间。
//
// 只在 count==1 时设过期：若每次都续期，持续有流量的 key 会一直不失效，
// 把“固定窗口”变成“永不释放”。设置过期失败时主动删除 key 并返回错误，
// 让上层 fail-open，避免留下一个永不淘汰的计数把客户端永久封禁。
func (c *RedisCounter) Incr(ctx context.Context, key string, window time.Duration) (int64, error) {
	k := "rl:" + key
	count, err := c.r.client.Incr(ctx, k).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		if e := c.r.client.Expire(ctx, k, window).Err(); e != nil {
			c.r.client.Del(ctx, k)
			return 0, e
		}
	}
	return count, nil
}
