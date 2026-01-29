package httpx

import (
	"net/http"
	"time"
)

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, m ...Middleware) http.Handler {
	// Apply in reverse so Chain(h, a, b) becomes a(b(h)).
	for i := len(m) - 1; i >= 0; i-- {
		h = m[i](h)
	}
	return h
}

func WithBodyLimit(limitBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limitBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func WithTimeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, "request timed out")
	}
}
