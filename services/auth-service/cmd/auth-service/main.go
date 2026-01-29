package main

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/config"
	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/libs/httpx"
	"github.com/md-rashed-zaman/apptremind/libs/kafkax"
	otelx "github.com/md-rashed-zaman/apptremind/libs/otel"
	"github.com/md-rashed-zaman/apptremind/libs/runtime"
	"github.com/md-rashed-zaman/apptremind/services/auth-service/internal/audit"
	"github.com/md-rashed-zaman/apptremind/services/auth-service/internal/handlers"
	"github.com/md-rashed-zaman/apptremind/services/auth-service/internal/outbox"
	"github.com/md-rashed-zaman/apptremind/services/auth-service/internal/sessions"
	"github.com/md-rashed-zaman/apptremind/services/auth-service/internal/storage"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	service := config.String("SERVICE_NAME", "auth-service")
	port, err := config.Port("PORT", "8081")
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

	dbURL, err := config.RequiredString("DATABASE_URL")
	if err != nil {
		panic(err)
	}

	pool, err := db.Open(ctx, dbURL)
	if err != nil {
		logger.Error("db connection failed", "err", err)
		panic(err)
	}
	defer pool.Close()

	mux := runtime.NewBaseMuxWithReady(
		runtime.ReadyCheck{Name: "db", Check: db.ReadyCheck(pool)},
		runtime.ReadyCheck{Name: "kafka", Check: kafkax.ReadyCheck(config.String("KAFKA_BROKERS", ""))},
	)
	userRepo := storage.NewUserRepository(pool)
	auditRepo := audit.NewRepository(pool)
	outboxRepo := outbox.NewRepository(pool)
	refreshRepo := sessions.NewRefreshRepository(pool)
	outboxPublisher := outbox.NewPublisher(pool, outboxRepo, logger, outbox.PublisherConfig{
		Brokers:   config.String("KAFKA_BROKERS", ""),
		PollEvery: 2 * time.Second,
		BatchSize: 50,
	})
	go outboxPublisher.Run(ctx)
	signer, err := buildSigner()
	if err != nil {
		logger.Error("failed to init jwt signer", "err", err)
		panic(err)
	}

	refreshTTLHours, err := strconv.Atoi(config.String("REFRESH_TTL_HOURS", "720"))
	if err != nil || refreshTTLHours <= 0 {
		logger.Error("invalid refresh ttl hours", "value", refreshTTLHours, "err", err)
		panic(err)
	}
	refreshTTL := time.Duration(refreshTTLHours) * time.Hour

	authHandler := handlers.NewAuthHandler(signer, pool, userRepo, auditRepo, outboxRepo, refreshRepo, refreshTTL)
	mux.HandleFunc("/api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("/api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("/api/v1/auth/refresh", authHandler.Refresh)
	mux.HandleFunc("/api/v1/auth/logout", authHandler.Logout)
	mux.HandleFunc("/api/v1/auth/me", authHandler.Me)
	mux.HandleFunc("/.well-known/jwks.json", authHandler.JWKS)
	mux.HandleFunc("/api/v1/auth/rotate", authHandler.Rotate)
	mux.HandleFunc("/api/v1/auth/audit", authHandler.Audit)
	handler := httpx.Chain(mux,
		httpx.WithRequestID,
		httpx.WithAccessLog(logger),
	)
	handler = otelhttp.NewHandler(handler, "auth")
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

func buildSigner() (handlers.TokenSigner, error) {
	privatePEM := config.String("JWT_PRIVATE_KEY_PEM", "")
	privatePEMS := config.String("JWT_PRIVATE_KEYS_PEM", "")
	activeKID := config.String("JWT_ACTIVE_KID", "")

	if privatePEMS != "" {
		keySet, err := handlers.ParseRS256KeySet(privatePEMS)
		if err != nil {
			return nil, err
		}
		signer, err := handlers.NewRotatingRS256Signer(keySet, activeKID)
		if err != nil {
			return nil, err
		}
		if rk := config.String("JWT_ROTATE_KEY", ""); rk != "" {
			if rotator, ok := signer.(*handlers.RotatingSigner); ok {
				rotator.SetRotateKey(rk)
			}
		}
		return signer, nil
	}
	if privatePEM != "" {
		return handlers.NewRS256Signer([]byte(privatePEM), config.String("JWT_KID", ""))
	}
	return handlers.NewHS256Signer(config.String("JWT_SECRET", "dev-secret")), nil
}
