package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/config"
	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/libs/httpx"
	"github.com/md-rashed-zaman/apptremind/libs/kafkax"
	otelx "github.com/md-rashed-zaman/apptremind/libs/otel"
	"github.com/md-rashed-zaman/apptremind/libs/runtime"
	"github.com/md-rashed-zaman/apptremind/services/scheduler-service/internal/consumer"
	"github.com/md-rashed-zaman/apptremind/services/scheduler-service/internal/inbox"
	"github.com/md-rashed-zaman/apptremind/services/scheduler-service/internal/jobs"
	"github.com/md-rashed-zaman/apptremind/services/scheduler-service/internal/outbox"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	service := config.String("SERVICE_NAME", "scheduler-service")
	port, err := config.Port("PORT", "8087")
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

	inboxRepo := inbox.NewRepository(pool)
	jobRepo := jobs.NewRepository()
	outboxRepo := outbox.NewRepository(pool)

	outboxPublisher := outbox.NewPublisher(pool, outboxRepo, logger, outbox.PublisherConfig{
		Brokers:   config.String("KAFKA_BROKERS", ""),
		PollEvery: 2 * time.Second,
		BatchSize: 50,
	})
	go outboxPublisher.Run(ctx)

	backoffSeconds, err := strconv.Atoi(config.String("SCHEDULER_BACKOFF_SECONDS", "60"))
	if err != nil || backoffSeconds <= 0 {
		backoffSeconds = 60
	}
	jobWorker := jobs.NewWorker(pool, jobRepo, outboxRepo, logger, jobs.WorkerConfig{
		Interval:  2 * time.Second,
		BatchSize: 50,
		Backoff:   time.Duration(backoffSeconds) * time.Second,
	})
	go jobWorker.Run(ctx)

	consumerCfg := consumer.Config{
		Brokers: config.String("KAFKA_BROKERS", ""),
		GroupID: config.String("KAFKA_GROUP_ID", "scheduler-service"),
		Topic:   config.String("KAFKA_CONSUME_TOPIC", "booking.reminder.requested.v1"),
	}

	type reminderRequest struct {
		AppointmentID string         `json:"appointment_id"`
		BusinessID    string         `json:"business_id"`
		Channel       string         `json:"channel"`
		Recipient     string         `json:"recipient"`
		RemindAt      string         `json:"remind_at"`
		TemplateData  map[string]any `json:"template_data"`
	}

	eventConsumer := consumer.New(logger, inboxRepo, consumerCfg, func(ctx context.Context, msg kafka.Message) error {
		var payload reminderRequest
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			logger.Error("invalid reminder request", "err", err)
			return nil
		}
		if payload.AppointmentID == "" || payload.BusinessID == "" || payload.Channel == "" || payload.Recipient == "" || payload.RemindAt == "" {
			logger.Error("missing reminder fields")
			return nil
		}
		remindAt, err := time.Parse(time.RFC3339, payload.RemindAt)
		if err != nil {
			logger.Error("invalid remind_at", "err", err)
			return nil
		}

		idempotencyKey := payload.AppointmentID + "|" + payload.RemindAt + "|" + payload.Channel

		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()

		if err := jobRepo.Insert(ctx, tx, jobs.Job{
			IdempotencyKey: idempotencyKey,
			AppointmentID:  payload.AppointmentID,
			BusinessID:     payload.BusinessID,
			Channel:        payload.Channel,
			Recipient:      payload.Recipient,
			RemindAt:       remindAt,
			TemplateData:   payload.TemplateData,
		}); err != nil {
			return err
		}

		return tx.Commit(ctx)
	})
	go eventConsumer.Run(ctx)

	mux := runtime.NewBaseMuxWithReady(
		runtime.ReadyCheck{Name: "db", Check: db.ReadyCheck(pool)},
		runtime.ReadyCheck{Name: "kafka", Check: kafkax.ReadyCheck(config.String("KAFKA_BROKERS", ""))},
	)
	handler := httpx.Chain(mux,
		httpx.WithRequestID,
		httpx.WithAccessLog(logger),
	)
	handler = otelhttp.NewHandler(handler, "scheduler")
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
