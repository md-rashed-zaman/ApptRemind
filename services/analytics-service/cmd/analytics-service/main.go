package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/config"
	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/libs/httpx"
	"github.com/md-rashed-zaman/apptremind/libs/kafkax"
	otelx "github.com/md-rashed-zaman/apptremind/libs/otel"
	"github.com/md-rashed-zaman/apptremind/libs/runtime"
	"github.com/md-rashed-zaman/apptremind/services/analytics-service/internal/consumer"
	"github.com/md-rashed-zaman/apptremind/services/analytics-service/internal/inbox"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	service := config.String("SERVICE_NAME", "analytics-service")
	port, err := config.Port("PORT", "8086")
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
	sentConsumerCfg := consumer.Config{
		Brokers: config.String("KAFKA_BROKERS", ""),
		GroupID: config.String("KAFKA_GROUP_ID", "analytics-service"),
		Topic:   "notification.sent.v1",
	}
	sentConsumer := consumer.New(logger, inboxRepo, sentConsumerCfg, func(ctx context.Context, msg kafka.Message) error {
		var payload struct {
			AppointmentID string `json:"appointment_id"`
			BusinessID    string `json:"business_id"`
			Channel       string `json:"channel"`
			SentAt        string `json:"sent_at"`
		}

		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			logger.Error("invalid event payload", "err", err)
			return nil
		}
		if payload.AppointmentID == "" || payload.Channel == "" || payload.SentAt == "" {
			logger.Error("missing event fields")
			return nil
		}
		if _, err := time.Parse(time.RFC3339, payload.SentAt); err != nil {
			logger.Error("invalid sent_at", "err", err)
			return nil
		}

		_, err := pool.Exec(ctx, `
			INSERT INTO notification_metrics (appointment_id, business_id, channel, sent_at, status)
			VALUES ($1, NULLIF($2, '')::uuid, $3, $4, 'sent')
		`, payload.AppointmentID, payload.BusinessID, payload.Channel, payload.SentAt)
		if err != nil {
			logger.Error("failed to write metrics", "err", err)
			return err
		}

		if payload.BusinessID != "" {
			if err := bumpNotificationAggregate(ctx, pool, payload.BusinessID, payload.Channel, payload.SentAt, 1, 0); err != nil {
				logger.Error("failed to update daily notification metrics", "err", err)
				return err
			}
		}

		logger.Info("notification metric recorded", "appointment_id", payload.AppointmentID, "channel", payload.Channel)
		return nil
	})
	go sentConsumer.Run(ctx)

	failedConsumerCfg := consumer.Config{
		Brokers: config.String("KAFKA_BROKERS", ""),
		GroupID: config.String("KAFKA_GROUP_ID", "analytics-service"),
		Topic:   "notification.failed.v1",
	}
	failedConsumer := consumer.New(logger, inboxRepo, failedConsumerCfg, func(ctx context.Context, msg kafka.Message) error {
		var payload struct {
			AppointmentID string `json:"appointment_id"`
			BusinessID    string `json:"business_id"`
			Channel       string `json:"channel"`
			ErrorReason   string `json:"error_reason"`
			FailedAt      string `json:"failed_at"`
		}

		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			logger.Error("invalid failed payload", "err", err)
			return nil
		}
		if payload.AppointmentID == "" || payload.Channel == "" || payload.ErrorReason == "" || payload.FailedAt == "" {
			logger.Error("missing failed fields")
			return nil
		}
		if _, err := time.Parse(time.RFC3339, payload.FailedAt); err != nil {
			logger.Error("invalid failed_at", "err", err)
			return nil
		}

		_, err := pool.Exec(ctx, `
			INSERT INTO notification_metrics (appointment_id, business_id, channel, sent_at, status)
			VALUES ($1, NULLIF($2, '')::uuid, $3, $4, 'failed')
		`, payload.AppointmentID, payload.BusinessID, payload.Channel, payload.FailedAt)
		if err != nil {
			logger.Error("failed to write failed metrics", "err", err)
			return err
		}

		if payload.BusinessID != "" {
			if err := bumpNotificationAggregate(ctx, pool, payload.BusinessID, payload.Channel, payload.FailedAt, 0, 1); err != nil {
				logger.Error("failed to update daily notification metrics", "err", err)
				return err
			}
		}

		logger.Info("notification failure recorded", "appointment_id", payload.AppointmentID, "channel", payload.Channel)
		return nil
	})
	go failedConsumer.Run(ctx)

	dlqConsumerCfg := consumer.Config{
		Brokers: config.String("KAFKA_BROKERS", ""),
		GroupID: config.String("KAFKA_GROUP_ID", "analytics-service"),
		Topic:   "scheduler.reminder.dlq.v1",
	}
	dlqConsumer := consumer.New(logger, inboxRepo, dlqConsumerCfg, func(ctx context.Context, msg kafka.Message) error {
		var payload struct {
			AppointmentID string `json:"appointment_id"`
			BusinessID    string `json:"business_id"`
			Channel       string `json:"channel"`
			Recipient     string `json:"recipient"`
			RemindAt      string `json:"remind_at"`
			ErrorReason   string `json:"error_reason"`
			FailedAt      string `json:"failed_at"`
		}

		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			logger.Error("invalid dlq payload", "err", err)
			return nil
		}
		if payload.AppointmentID == "" || payload.BusinessID == "" || payload.Channel == "" || payload.Recipient == "" || payload.RemindAt == "" || payload.ErrorReason == "" || payload.FailedAt == "" {
			logger.Error("missing dlq fields")
			return nil
		}
		if _, err := time.Parse(time.RFC3339, payload.FailedAt); err != nil {
			logger.Error("invalid failed_at", "err", err)
			return nil
		}

		remindAt, err := time.Parse(time.RFC3339, payload.RemindAt)
		if err != nil {
			logger.Error("invalid remind_at", "err", err)
			return nil
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO scheduler_dlq_events (appointment_id, business_id, channel, recipient, remind_at, error_reason, failed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, payload.AppointmentID, payload.BusinessID, payload.Channel, payload.Recipient, remindAt, payload.ErrorReason, payload.FailedAt)
		if err != nil {
			logger.Error("failed to write dlq event", "err", err)
			return err
		}

		logger.Warn("scheduler dlq recorded", "appointment_id", payload.AppointmentID, "channel", payload.Channel)
		return nil
	})
	go dlqConsumer.Run(ctx)

	authAuditCfg := consumer.Config{
		Brokers: config.String("KAFKA_BROKERS", ""),
		GroupID: config.String("KAFKA_GROUP_ID", "analytics-service"),
		Topic:   "auth.audit.v1",
	}
	authAuditConsumer := consumer.New(logger, inboxRepo, authAuditCfg, func(ctx context.Context, msg kafka.Message) error {
		var payload struct {
			EventType string          `json:"event_type"`
			ActorID   string          `json:"actor_id"`
			Metadata  json.RawMessage `json:"metadata"`
			CreatedAt string          `json:"created_at"`
		}

		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			logger.Error("invalid auth audit payload", "err", err)
			return nil
		}
		if payload.EventType == "" || payload.CreatedAt == "" {
			logger.Error("missing auth audit fields")
			return nil
		}
		if _, err := time.Parse(time.RFC3339, payload.CreatedAt); err != nil {
			logger.Error("invalid auth audit created_at", "err", err)
			return nil
		}

		_, err := pool.Exec(ctx, `
			INSERT INTO security_audit_events (event_type, actor_id, metadata, created_at)
			VALUES ($1, NULLIF($2, ''), $3, $4)
		`, payload.EventType, payload.ActorID, payload.Metadata, payload.CreatedAt)
		if err != nil {
			logger.Error("failed to write security audit event", "err", err)
			return err
		}

		logger.Info("security audit recorded", "event_type", payload.EventType)
		return nil
	})
	go authAuditConsumer.Run(ctx)

	handleBookingEvent := func(ctx context.Context, msg kafka.Message, kind string) error {
		var payload struct {
			AppointmentID string `json:"appointment_id"`
			BusinessID    string `json:"business_id"`
			StartTime     string `json:"start_time"`
			EndTime       string `json:"end_time"`
			CancelledAt   string `json:"cancelled_at"`
		}

		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			logger.Error("invalid booking payload", "err", err)
			return nil
		}
		if payload.AppointmentID == "" || payload.BusinessID == "" || payload.StartTime == "" {
			logger.Error("missing booking fields")
			return nil
		}
		startTime, err := time.Parse(time.RFC3339, payload.StartTime)
		if err != nil {
			logger.Error("invalid start_time", "err", err)
			return nil
		}

		meta := kafkax.ExtractEventMeta(msg)

		tx, err := pool.Begin(ctx)
		if err != nil {
			logger.Error("db begin failed", "err", err)
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()

		tag, err := tx.Exec(ctx, `
			INSERT INTO booking_events (event_id, event_type, business_id, appointment_id, occurred_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (event_id) DO NOTHING
		`, meta.EventID, meta.EventType, payload.BusinessID, payload.AppointmentID, startTime.UTC())
		if err != nil {
			logger.Error("failed to insert booking event", "err", err)
			return err
		}
		if tag.RowsAffected() == 0 {
			_ = tx.Commit(ctx)
			return nil
		}

		bookedInc := 0
		canceledInc := 0
		if kind == "booked" {
			bookedInc = 1
		} else if kind == "canceled" {
			canceledInc = 1
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO daily_appointment_metrics (business_id, day, booked_count, canceled_count)
			VALUES ($1, $2::date, $3, $4)
			ON CONFLICT (business_id, day)
			DO UPDATE SET booked_count = daily_appointment_metrics.booked_count + EXCLUDED.booked_count,
			              canceled_count = daily_appointment_metrics.canceled_count + EXCLUDED.canceled_count,
			              updated_at = now()
		`, payload.BusinessID, startTime.UTC(), bookedInc, canceledInc); err != nil {
			logger.Error("failed to update daily metrics", "err", err)
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			logger.Error("failed to commit booking metric", "err", err)
			return err
		}

		logger.Info("booking metric recorded", "appointment_id", payload.AppointmentID, "business_id", payload.BusinessID, "event_type", meta.EventType)
		return nil
	}

	bookedConsumerCfg := consumer.Config{
		Brokers: config.String("KAFKA_BROKERS", ""),
		GroupID: config.String("KAFKA_GROUP_ID", "analytics-service"),
		Topic:   "booking.appointment.booked.v1",
	}
	bookedConsumer := consumer.New(logger, inboxRepo, bookedConsumerCfg, func(ctx context.Context, msg kafka.Message) error {
		return handleBookingEvent(ctx, msg, "booked")
	})
	go bookedConsumer.Run(ctx)

	cancelConsumerCfg := consumer.Config{
		Brokers: config.String("KAFKA_BROKERS", ""),
		GroupID: config.String("KAFKA_GROUP_ID", "analytics-service"),
		Topic:   "booking.appointment.cancelled.v1",
	}
	cancelConsumer := consumer.New(logger, inboxRepo, cancelConsumerCfg, func(ctx context.Context, msg kafka.Message) error {
		return handleBookingEvent(ctx, msg, "canceled")
	})
	go cancelConsumer.Run(ctx)

	mux := runtime.NewBaseMuxWithReady(
		runtime.ReadyCheck{Name: "db", Check: db.ReadyCheck(pool)},
		runtime.ReadyCheck{Name: "kafka", Check: kafkax.ReadyCheck(config.String("KAFKA_BROKERS", ""))},
	)
	handler := httpx.Chain(mux,
		httpx.WithRequestID,
		httpx.WithAccessLog(logger),
	)
	handler = otelhttp.NewHandler(handler, "analytics")
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

func bumpNotificationAggregate(ctx context.Context, pool *db.Pool, businessID, channel, ts string, sentInc, failedInc int) error {
	if businessID == "" || channel == "" || ts == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return nil
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO daily_notification_metrics (business_id, day, channel, sent_count, failed_count)
		VALUES ($1, $2::date, $3, $4, $5)
		ON CONFLICT (business_id, day, channel)
		DO UPDATE SET sent_count = daily_notification_metrics.sent_count + EXCLUDED.sent_count,
		              failed_count = daily_notification_metrics.failed_count + EXCLUDED.failed_count,
		              updated_at = now()
	`, businessID, t.UTC(), channel, sentInc, failedInc)
	return err
}
