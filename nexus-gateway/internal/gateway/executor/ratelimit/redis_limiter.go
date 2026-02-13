package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLimiter struct {
	client *redis.Client
	prefix string
}

func NewRedisLimiter(redisURL string) (*RedisLimiter, func(), error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, nil, err
	}
	c := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, nil, err
	}
	closeFn := func() {
		_ = c.Close()
	}
	return &RedisLimiter{
		client: c,
		prefix: "nexus:ratelimit",
	}, closeFn, nil
}

func (l *RedisLimiter) Allow(key string, perMinute int) bool {
	if perMinute <= 0 {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	window := time.Now().UTC().Format("200601021504")
	redisKey := fmt.Sprintf("%s:%s:%s", l.prefix, key, window)

	count, err := l.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return false
	}
	if count == 1 {
		_ = l.client.Expire(ctx, redisKey, time.Minute+5*time.Second).Err()
	}
	return count <= int64(perMinute)
}
