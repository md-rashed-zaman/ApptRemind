package main

import (
	"context"
	"embed"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/auth"
	"github.com/md-rashed-zaman/apptremind/libs/config"
	"github.com/md-rashed-zaman/apptremind/libs/httpx"
	otelx "github.com/md-rashed-zaman/apptremind/libs/otel"
	"github.com/md-rashed-zaman/apptremind/libs/runtime"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

//go:embed assets/gateway.v1.yaml
var openAPISpec embed.FS

func main() {
	service := config.String("SERVICE_NAME", "gateway-service")
	port, err := config.Port("PORT", "8080")
	if err != nil {
		panic(err)
	}
	logger := runtime.NewLogger(service)

	ctx, stop := runtime.SignalContext()
	defer stop()

	otelShutdown, err := otelx.Setup(ctx, otelx.ConfigFromEnv(service))
	if err != nil {
		logger.Error("otel setup failed", "err", err)
	} else {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = otelShutdown(shutdownCtx)
		}()
	}

	mux := runtime.NewBaseMuxWithReady()
	jwtSecret := config.String("JWT_SECRET", "dev-secret")
	jwksURL := config.String("JWKS_URL", "")
	jwksTTL, err := strconv.Atoi(config.String("JWKS_CACHE_SECONDS", "300"))
	if err != nil || jwksTTL <= 0 {
		jwksTTL = 300
	}
	registerRoutes(mux, jwtSecret, jwksURL, time.Duration(jwksTTL)*time.Second)

	bodyLimit := int64(1 << 20) // 1MB
	if v, err := strconv.Atoi(config.String("REQUEST_BODY_LIMIT_BYTES", "1048576")); err == nil && v > 0 {
		bodyLimit = int64(v)
	}
	requestTimeout := 10 * time.Second
	if v, err := strconv.Atoi(config.String("REQUEST_TIMEOUT_SECONDS", "10")); err == nil && v > 0 {
		requestTimeout = time.Duration(v) * time.Second
	}

	limitPerMinute := 60
	if v, err := strconv.Atoi(config.String("RATE_LIMIT_PER_MINUTE", "60")); err == nil && v > 0 {
		limitPerMinute = v
	}

	var rateLimitMW httpx.Middleware
	if addr := strings.TrimSpace(config.String("REDIS_ADDR", "")); addr != "" {
		redisDB := 0
		if v, err := strconv.Atoi(config.String("REDIS_DB", "0")); err == nil && v >= 0 {
			redisDB = v
		}
		rdb := redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: config.String("REDIS_PASSWORD", ""),
			DB:       redisDB,
		})
		defer func() { _ = rdb.Close() }()

		rl := httpx.NewRedisRateLimiter(rdb, limitPerMinute, time.Minute, config.String("RATE_LIMIT_PREFIX", "rl"))
		rateLimitMW = rl.Middleware(logger, isTruthy(config.String("RATE_LIMIT_FAIL_OPEN", "true")))
		logger.Info("rate limiting enabled (redis)", "per_minute", limitPerMinute, "redis_addr", addr)
	} else {
		rl := httpx.NewRateLimiter(limitPerMinute, time.Minute)
		rateLimitMW = rl.Middleware()
		logger.Info("rate limiting enabled (in-memory)", "per_minute", limitPerMinute)
	}

	handler := httpx.Chain(mux,
		httpx.WithCORS(httpx.CORSPolicy{
			AllowedOrigins:   parseList(config.String("CORS_ALLOWED_ORIGINS", "")),
			AllowedMethods:   parseList(config.String("CORS_ALLOWED_METHODS", "GET,POST,PUT,PATCH,DELETE,OPTIONS")),
			AllowedHeaders:   parseList(config.String("CORS_ALLOWED_HEADERS", "Authorization,Content-Type,X-Request-Id,X-Idempotency-Key")),
			AllowCredentials: isTruthy(config.String("CORS_ALLOW_CREDENTIALS", "false")),
			MaxAge:           time.Duration(corsMaxAgeSeconds()) * time.Second,
		}),
		httpx.WithRequestID,
		httpx.WithAccessLog(logger),
		httpx.WithBodyLimit(bodyLimit),
		httpx.WithTimeout(requestTimeout),
		rateLimitMW,
	)
	handler = otelhttp.NewHandler(handler, "gateway")
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("http server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", "err", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", "err", err)
	}
	logger.Info("http server stopped")
}

func isTruthy(s string) bool {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func parseList(raw string) []string {
	items := strings.Split(raw, ",")
	out := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func corsMaxAgeSeconds() int {
	value := 600
	if v, err := strconv.Atoi(config.String("CORS_MAX_AGE_SECONDS", "600")); err == nil && v > 0 {
		value = v
	}
	return value
}

func registerRoutes(mux *http.ServeMux, jwtSecret string, jwksURL string, jwksTTL time.Duration) {
	authURL := mustParseURL(config.String("AUTH_URL", "http://auth-service:8081"))
	businessURL := mustParseURL(config.String("BUSINESS_URL", "http://business-service:8082"))
	bookingURL := mustParseURL(config.String("BOOKING_URL", "http://booking-service:8083"))
	billingURL := mustParseURL(config.String("BILLING_URL", "http://billing-service:8084"))

	authProxy := httputil.NewSingleHostReverseProxy(authURL)
	businessProxy := httputil.NewSingleHostReverseProxy(businessURL)
	bookingProxy := httputil.NewSingleHostReverseProxy(bookingURL)
	billingProxy := httputil.NewSingleHostReverseProxy(billingURL)
	otelTransport := otelhttp.NewTransport(http.DefaultTransport)
	authProxy.Transport = otelTransport
	businessProxy.Transport = otelTransport
	bookingProxy.Transport = otelTransport
	billingProxy.Transport = otelTransport

	var jwksClient *auth.JWKSClient
	if jwksURL != "" {
		jwksClient = auth.NewJWKSClient(jwksURL, jwksTTL)
	}

	registerProxy(mux, "/api/v1/auth", authProxy)
	registerProxy(mux, "/api/v1/public", bookingProxy)
	registerProxy(mux, "/api/v1/business", requireAuth(requireRole(businessProxy, "owner", "admin"), jwtSecret, jwksClient))
	registerProxy(mux, "/api/v1/appointments", requireAuth(bookingProxy, jwtSecret, jwksClient))
	// Stripe needs to reach the webhook endpoint without a JWT; signature verification is the auth.
	registerProxy(mux, "/api/v1/billing/webhooks/stripe", billingProxy)
	// Checkout return page can poll this without a JWT.
	registerProxy(mux, "/api/v1/billing/checkout/session", billingProxy)
	registerProxy(mux, "/api/v1/billing/checkout/session/ack", billingProxy)
	registerProxy(mux, "/api/v1/billing", requireAuth(requireRole(billingProxy, "owner", "admin"), jwtSecret, jwksClient))
	registerProxy(mux, "/.well-known/jwks.json", authProxy)

	mux.HandleFunc("/billing/success", func(w http.ResponseWriter, r *http.Request) {
		renderCheckoutReturnPage(w, r, "Payment successful", "success")
	})
	mux.HandleFunc("/billing/cancel", func(w http.ResponseWriter, r *http.Request) {
		renderCheckoutReturnPage(w, r, "Payment canceled", "cancel")
	})

	mux.HandleFunc("/openapi", func(w http.ResponseWriter, _ *http.Request) {
		data, err := openAPISpec.ReadFile("assets/gateway.v1.yaml")
		if err != nil {
			http.Error(w, "openapi not available", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
}

func renderCheckoutReturnPage(w http.ResponseWriter, r *http.Request, title string, mode string) {
	sessionID := r.URL.Query().Get("session_id")
	state := r.URL.Query().Get("state")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	// Keep it dependency-free; this is just a local/prod skeleton until a real frontend exists.
	_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8">`))
	_, _ = w.Write([]byte(`<meta name="viewport" content="width=device-width, initial-scale=1">`))
	_, _ = w.Write([]byte(`<title>` + title + `</title>`))
	_, _ = w.Write([]byte(`<style>body{font-family:system-ui,-apple-system,Segoe UI,Roboto,Ubuntu,Arial,sans-serif;margin:40px;max-width:880px;line-height:1.4}code{background:#f4f4f4;padding:2px 4px;border-radius:4px}pre{background:#0b1020;color:#e6edf3;padding:12px;border-radius:8px;overflow:auto}</style>`))
	_, _ = w.Write([]byte(`</head><body>`))
	_, _ = w.Write([]byte(`<h1>` + title + `</h1>`))
	if sessionID == "" {
		_, _ = w.Write([]byte(`<p>Missing <code>session_id</code> query parameter.</p>`))
		_, _ = w.Write([]byte(`</body></html>`))
		return
	}
	_, _ = w.Write([]byte(`<p>Session: <code>` + htmlEscape(sessionID) + `</code></p>`))
	_, _ = w.Write([]byte(`<p>Status: <span id="status">checking...</span></p>`))
	_, _ = w.Write([]byte(`<pre id="raw"></pre>`))
	_, _ = w.Write([]byte(`<script>
const sessionId = ` + "`" + htmlEscape(sessionID) + "`" + `;
const state = ` + "`" + htmlEscape(state) + "`" + `;
const mode = ` + "`" + mode + "`" + `;
async function ack() {
  if (!state) return;
  try {
    await fetch('/api/v1/billing/checkout/session/ack', {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({session_id: sessionId, state: state, result: mode}),
    });
  } catch (e) {}
}
async function poll() {
  try {
    const resp = await fetch('/api/v1/billing/checkout/session?session_id=' + encodeURIComponent(sessionId), {cache:'no-store'});
    const txt = await resp.text();
    let obj = null;
    try { obj = JSON.parse(txt); } catch (e) {}
    document.getElementById('raw').textContent = txt;
    if (!resp.ok) {
      document.getElementById('status').textContent = 'error (' + resp.status + ')';
      return;
    }
    const s = obj && obj.status ? obj.status : 'unknown';
    document.getElementById('status').textContent = s;
    if (mode === 'success' && s !== 'completed') setTimeout(poll, 1500);
  } catch (e) {
    document.getElementById('status').textContent = 'error';
  }
}
ack();
poll();
</script>`))
	_, _ = w.Write([]byte(`</body></html>`))
}

func htmlEscape(s string) string {
	// Minimal escaping for our use case (query string reflected in HTML/JS).
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, `'`, "&#39;")
	return s
}

func registerProxy(mux *http.ServeMux, prefix string, handler http.Handler) {
	if !strings.HasSuffix(prefix, "/") {
		mux.Handle(prefix, handler)
		mux.Handle(prefix+"/", handler)
		return
	}
	mux.Handle(prefix, handler)
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func requireAuth(next http.Handler, jwtSecret string, jwksClient *auth.JWKSClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") || len(strings.TrimSpace(authHeader)) <= len("Bearer ") {
			http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		var claims *auth.Claims
		var err error

		if jwksClient != nil {
			header, err := auth.ParseHeader(token)
			if err != nil {
				http.Error(w, "invalid token header", http.StatusUnauthorized)
				return
			}
			if header.Alg == "RS256" && header.Kid != "" {
				pub, err := jwksClient.Get(header.Kid)
				if err != nil {
					http.Error(w, "invalid token key", http.StatusUnauthorized)
					return
				}
				claims, err = auth.VerifyRS256(token, pub)
			} else {
				claims, err = auth.ParseAndVerifyHS256(token, jwtSecret)
			}
		} else {
			claims, err = auth.ParseAndVerifyHS256(token, jwtSecret)
		}
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		r.Header.Del("X-User-Id")
		r.Header.Del("X-Business-Id")
		r.Header.Del("X-Role")
		r.Header.Set("X-User-Id", claims.Sub)
		r.Header.Set("X-Business-Id", claims.BusinessID)
		r.Header.Set("X-Role", claims.Role)
		next.ServeHTTP(w, r)
	})
}

func requireRole(next http.Handler, roles ...string) http.Handler {
	allowed := map[string]struct{}{}
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := r.Header.Get("X-Role")
		if _, ok := allowed[role]; !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
