package httpx

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter is a fixed-window rate limiter backed by Redis.
// It is meant for production deployments where multiple gateway instances run concurrently.
type RedisRateLimiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
	prefix string
}

var redisFixedWindowScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`)

func NewRedisRateLimiter(rdb *redis.Client, limit int, window time.Duration, prefix string) *RedisRateLimiter {
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "rl"
	}
	return &RedisRateLimiter{rdb: rdb, limit: limit, window: window, prefix: prefix}
}

func (rl *RedisRateLimiter) Middleware(logger *slog.Logger, failOpen bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rl.prefix + ":" + clientKey(r)
			count, err := rl.incr(r.Context(), key)
			if err != nil {
				if logger != nil {
					logger.Warn("redis rate limiter error", "err", err)
				}
				if failOpen {
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, "rate limiter unavailable", http.StatusServiceUnavailable)
				return
			}
			if count > int64(rl.limit) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RedisRateLimiter) incr(ctx context.Context, key string) (int64, error) {
	ms := rl.window.Milliseconds()
	if ms <= 0 {
		ms = int64(time.Minute / time.Millisecond)
	}
	res, err := redisFixedWindowScript.Run(ctx, rl.rdb, []string{key}, ms).Result()
	if err != nil {
		return 0, err
	}
	switch v := res.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		// Lua sometimes returns strings depending on Redis config/driver conversions.
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, err
		}
		return n, nil
	default:
		return 0, fmt.Errorf("unexpected redis script result type %T", res)
	}
}
