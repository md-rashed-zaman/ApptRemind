package main

import (
	"context"
	"encoding/json"
	"log/slog"
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
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/consumer"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/handlers"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/inbox"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/outbox"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/policy"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/scheduling"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/storage"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func parseReminderOffsets(raw string, logger *slog.Logger) []time.Duration {
	var offsets []time.Duration
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		mins, err := strconv.Atoi(part)
		if err != nil || mins <= 0 {
			logger.Warn("invalid reminder offset", "value", part)
			continue
		}
		offsets = append(offsets, time.Duration(mins)*time.Minute)
	}
	if len(offsets) == 0 {
		offsets = []time.Duration{24 * time.Hour}
	}
	return offsets
}

func main() {
	service := config.String("SERVICE_NAME", "booking-service")
	port, err := config.Port("PORT", "8083")
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

	repo := storage.NewBookingRepository(pool)
	outboxRepo := outbox.NewRepository(pool)
	offsets := parseReminderOffsets(config.String("REMINDER_OFFSETS_MINUTES", "1440,60"), logger)
	policyProvider, err := policy.NewBusinessPolicyProvider(logger, offsets, config.String("BUSINESS_GRPC_ADDR", ""))
	if err != nil {
		logger.Error("policy provider init failed", "err", err)
		policyProvider = policy.NewStaticProvider(offsets)
	}
	schedulingProvider, err := scheduling.NewProvider(config.String("BUSINESS_GRPC_ADDR", ""))
	if err != nil {
		logger.Error("scheduling provider init failed; using fallback", "err", err)
		schedulingProvider = nil
	}
	outboxPublisher := outbox.NewPublisher(pool, outboxRepo, logger, outbox.PublisherConfig{
		Brokers:   config.String("KAFKA_BROKERS", ""),
		PollEvery: 2 * time.Second,
		BatchSize: 50,
	})
	go outboxPublisher.Run(ctx)

	inboxRepo := inbox.NewRepository(pool)
	startConsumer := func(topic string) {
		if strings.TrimSpace(topic) == "" {
			return
		}
		consumerCfg := consumer.Config{
			Brokers: config.String("KAFKA_BROKERS", ""),
			GroupID: config.String("KAFKA_GROUP_ID", "booking-service"),
			Topic:   topic,
		}
		eventConsumer := consumer.New(logger, inboxRepo, consumerCfg, func(ctx context.Context, msg kafka.Message) error {
			// Both events carry the same limit fields; booking enforces using this local cache.
			var payload struct {
				BusinessID             string `json:"business_id"`
				Tier                   string `json:"tier"`
				MaxMonthlyAppointments int    `json:"max_monthly_appointments"`
			}
			if err := json.Unmarshal(msg.Value, &payload); err != nil {
				logger.Error("invalid event payload", "err", err, "topic", msg.Topic)
				return nil
			}
			if payload.BusinessID == "" || payload.Tier == "" || payload.MaxMonthlyAppointments <= 0 {
				logger.Error("missing required event fields", "topic", msg.Topic)
				return nil
			}

			tx, err := pool.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = tx.Rollback(ctx) }()

			if err := repo.UpsertBusinessEntitlements(ctx, tx, storage.BusinessEntitlements{
				BusinessID:             payload.BusinessID,
				Tier:                   payload.Tier,
				MaxMonthlyAppointments: payload.MaxMonthlyAppointments,
			}); err != nil {
				return err
			}
			return tx.Commit(ctx)
		})
		go eventConsumer.Run(ctx)
	}

	startConsumer(config.String("KAFKA_CONSUME_TOPIC", "billing.subscription.activated.v1"))
	startConsumer(config.String("KAFKA_CONSUME_TOPIC_2", ""))
	bookingHandler := handlers.NewBookingHandler(repo, outboxRepo, logger, policyProvider, schedulingProvider, offsets)

	mux := runtime.NewBaseMuxWithReady(
		runtime.ReadyCheck{Name: "db", Check: db.ReadyCheck(pool)},
		runtime.ReadyCheck{Name: "kafka", Check: kafkax.ReadyCheck(config.String("KAFKA_BROKERS", ""))},
	)
	setupEntitlementsRoutes(ctx, mux, logger)
	mux.HandleFunc("/api/v1/public/slots", bookingHandler.Slots)
	mux.HandleFunc("/api/v1/public/book", bookingHandler.Create)
	mux.HandleFunc("/api/v1/appointments", bookingHandler.List)
	mux.HandleFunc("/api/v1/appointments/cancel", bookingHandler.Cancel)
	httpHandler := httpx.Chain(mux,
		httpx.WithRequestID,
		httpx.WithAccessLog(logger),
	)
	httpHandler = otelhttp.NewHandler(httpHandler, "booking")
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           httpHandler,
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
