package httpx

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// CORSPolicy defines the CORS headers to emit for matching origins.
type CORSPolicy struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// WithCORS adds basic CORS handling. If AllowedOrigins is empty, it is a no-op.
func WithCORS(cfg CORSPolicy) Middleware {
	if len(cfg.AllowedOrigins) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	allowedOrigins := normalizeList(cfg.AllowedOrigins)
	allowedMethods := strings.Join(normalizeList(cfg.AllowedMethods), ", ")
	allowedHeaders := strings.Join(normalizeList(cfg.AllowedHeaders), ", ")
	maxAge := int(cfg.MaxAge.Seconds())

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			allowOrigin, ok := matchOrigin(origin, allowedOrigins, cfg.AllowCredentials)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			headers := w.Header()
			headers.Set("Access-Control-Allow-Origin", allowOrigin)
			if cfg.AllowCredentials {
				headers.Set("Access-Control-Allow-Credentials", "true")
			}
			if allowedMethods != "" {
				headers.Set("Access-Control-Allow-Methods", allowedMethods)
			}
			if allowedHeaders != "" {
				headers.Set("Access-Control-Allow-Headers", allowedHeaders)
			}
			if maxAge > 0 {
				headers.Set("Access-Control-Max-Age", strconv.Itoa(maxAge))
			}
			headers.Add("Vary", "Origin")
			headers.Add("Vary", "Access-Control-Request-Method")
			headers.Add("Vary", "Access-Control-Request-Headers")

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func matchOrigin(origin string, allowed []string, allowCredentials bool) (string, bool) {
	for _, candidate := range allowed {
		if candidate == "*" {
			if allowCredentials {
				return origin, true
			}
			return "*", true
		}
		if strings.EqualFold(candidate, origin) {
			return origin, true
		}
	}
	return "", false
}
