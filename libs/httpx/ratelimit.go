package httpx

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	limit    int
	window   time.Duration
	mu       sync.Mutex
	visitors map[string]*visitor
}

type visitor struct {
	count     int
	resetTime time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:    limit,
		window:   window,
		visitors: map[string]*visitor{},
	}
}

func (rl *RateLimiter) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientKey(r)
			if !rl.allow(key) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v := rl.visitors[key]
	if v == nil || now.After(v.resetTime) {
		rl.visitors[key] = &visitor{
			count:     1,
			resetTime: now.Add(rl.window),
		}
		return true
	}

	if v.count >= rl.limit {
		return false
	}
	v.count++
	return true
}

func clientKey(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
