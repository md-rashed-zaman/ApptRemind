package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/config"
	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/libs/httpx"
	"github.com/md-rashed-zaman/apptremind/libs/kafkax"
	otelx "github.com/md-rashed-zaman/apptremind/libs/otel"
	"github.com/md-rashed-zaman/apptremind/libs/runtime"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/handlers"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/outbox"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/reconcile"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/storage"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/subscriptions"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	service := config.String("SERVICE_NAME", "billing-service")
	port, err := config.Port("PORT", "8084")
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

	repo := storage.NewRepository(pool)
	outboxRepo := outbox.NewRepository(pool)
	subSvc := subscriptions.New(repo, outboxRepo)
	outboxPublisher := outbox.NewPublisher(pool, outboxRepo, logger, outbox.PublisherConfig{
		Brokers:   config.String("KAFKA_BROKERS", ""),
		PollEvery: 2 * time.Second,
		BatchSize: 50,
	})
	go outboxPublisher.Run(ctx)

	mux := runtime.NewBaseMuxWithReady(
		runtime.ReadyCheck{Name: "db", Check: db.ReadyCheck(pool)},
		runtime.ReadyCheck{Name: "kafka", Check: kafkax.ReadyCheck(config.String("KAFKA_BROKERS", ""))},
	)
	tolSeconds, err := strconv.Atoi(config.String("STRIPE_WEBHOOK_TOLERANCE_SECONDS", "300"))
	if err != nil || tolSeconds <= 0 {
		tolSeconds = 300
	}
	h := handlers.New(repo, outboxRepo, logger, handlers.Config{
		StripeWebhookSecret:           config.String("STRIPE_WEBHOOK_SECRET", ""),
		StripeWebhookToleranceSeconds: tolSeconds,
		StripeSecretKey:               config.String("STRIPE_SECRET_KEY", ""),
		StripePriceStarter:            config.String("STRIPE_PRICE_STARTER", ""),
		StripePricePro:                config.String("STRIPE_PRICE_PRO", ""),
		CheckoutSuccessURL:            config.String("CHECKOUT_SUCCESS_URL", ""),
		CheckoutCancelURL:             config.String("CHECKOUT_CANCEL_URL", ""),
	})
	mux.HandleFunc("/api/v1/billing/checkout", h.CheckoutStub)
	mux.HandleFunc("/api/v1/billing/checkout/session", h.CheckoutSessionStatus)
	mux.HandleFunc("/api/v1/billing/checkout/session/ack", h.AckCheckoutReturn)
	mux.HandleFunc("/api/v1/billing/subscription", h.GetSubscription)
	mux.HandleFunc("/api/v1/billing/subscription/cancel", h.CancelSubscription)
	mux.HandleFunc("/api/v1/billing/webhooks/local", h.LocalWebhook)
	mux.HandleFunc("/api/v1/billing/webhooks/stripe", h.StripeWebhook)

	handler := httpx.Chain(mux,
		httpx.WithRequestID,
		httpx.WithAccessLog(logger),
	)
	handler = otelhttp.NewHandler(handler, "billing")
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

	// Stripe reconciliation: periodically self-heal subscription state if webhooks are missed.
	if isTruthy(config.String("BILLING_STRIPE_RECONCILE_ENABLED", "false")) {
		intervalSeconds, _ := strconv.Atoi(config.String("BILLING_STRIPE_RECONCILE_INTERVAL_SECONDS", "300"))
		if intervalSeconds <= 0 {
			intervalSeconds = 300
		}
		batchSize, _ := strconv.Atoi(config.String("BILLING_STRIPE_RECONCILE_BATCH_SIZE", "50"))
		lockKey, _ := strconv.ParseInt(config.String("BILLING_STRIPE_RECONCILE_LOCK_KEY", "4242001"), 10, 64)
		rec := reconcile.NewStripeReconciler(pool, repo, subSvc, logger, reconcile.StripeReconcilerConfig{
			StripeSecretKey: config.String("STRIPE_SECRET_KEY", ""),
			Interval:        time.Duration(intervalSeconds) * time.Second,
			BatchSize:       batchSize,
			AdvisoryLockKey: lockKey,
		})
		go rec.Run(ctx, time.Duration(intervalSeconds)*time.Second)
	}

	if err := startGrpcServer(ctx, logger, pool); err != nil {
		logger.Error("grpc server failed to start", "err", err)
	}

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
