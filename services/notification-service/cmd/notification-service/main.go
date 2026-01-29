package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/config"
	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/libs/httpx"
	"github.com/md-rashed-zaman/apptremind/libs/kafkax"
	otelx "github.com/md-rashed-zaman/apptremind/libs/otel"
	"github.com/md-rashed-zaman/apptremind/libs/runtime"
	"github.com/md-rashed-zaman/apptremind/services/notification-service/internal/consumer"
	"github.com/md-rashed-zaman/apptremind/services/notification-service/internal/email"
	"github.com/md-rashed-zaman/apptremind/services/notification-service/internal/inbox"
	"github.com/md-rashed-zaman/apptremind/services/notification-service/internal/outbox"
	"github.com/md-rashed-zaman/apptremind/services/notification-service/internal/sms"
	"github.com/md-rashed-zaman/apptremind/services/notification-service/internal/storage"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type reminderPayload struct {
	AppointmentID string         `json:"appointment_id"`
	BusinessID    string         `json:"business_id"`
	Channel       string         `json:"channel"`
	Recipient     string         `json:"recipient"`
	RemindAt      string         `json:"remind_at"`
	TemplateData  map[string]any `json:"template_data"`
}

func writeOutboxSent(ctx context.Context, pool *db.Pool, outboxRepo *outbox.Repository, payload reminderPayload, providerID string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if strings.TrimSpace(providerID) == "" {
		providerID = "unknown"
	}
	eventPayload, err := json.Marshal(map[string]any{
		"appointment_id": payload.AppointmentID,
		"business_id":    payload.BusinessID,
		"channel":        payload.Channel,
		"provider_id":    providerID,
		"sent_at":        time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	if err := outboxRepo.Insert(ctx, tx, outbox.Event{
		AggregateType: "notification",
		AggregateID:   payload.AppointmentID,
		EventType:     "notification.sent.v1",
		Payload:       eventPayload,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func writeOutboxFailed(ctx context.Context, pool *db.Pool, outboxRepo *outbox.Repository, payload reminderPayload, reason string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	eventPayload, err := json.Marshal(map[string]any{
		"appointment_id": payload.AppointmentID,
		"business_id":    payload.BusinessID,
		"channel":        payload.Channel,
		"error_reason":   reason,
		"failed_at":      time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	if err := outboxRepo.Insert(ctx, tx, outbox.Event{
		AggregateType: "notification",
		AggregateID:   payload.AppointmentID,
		EventType:     "notification.failed.v1",
		Payload:       eventPayload,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func main() {
	service := config.String("SERVICE_NAME", "notification-service")
	port, err := config.Port("PORT", "8085")
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
	notificationsRepo := storage.NewRepository(pool)
	outboxRepo := outbox.NewRepository(pool)
	outboxPublisher := outbox.NewPublisher(pool, outboxRepo, logger, outbox.PublisherConfig{
		Brokers:   config.String("KAFKA_BROKERS", ""),
		PollEvery: 2 * time.Second,
		BatchSize: 50,
	})
	go outboxPublisher.Run(ctx)

	smtpHost := config.String("SMTP_HOST", "mailpit")
	smtpPort := config.String("SMTP_PORT", "1025")
	smtpFrom := config.String("SMTP_FROM", "no-reply@apptremind.local")
	emailSender := email.NewSMTPSender(smtpHost, smtpPort, smtpFrom)
	emailProviderID := "smtp"

	smsProvider := strings.ToLower(config.String("SMS_PROVIDER", "noop"))
	smsWebhookURL := config.String("SMS_WEBHOOK_URL", "")
	smsWebhookToken := config.String("SMS_WEBHOOK_TOKEN", "")
	var smsSender sms.Sender
	switch smsProvider {
	case "webhook":
		smsSender = sms.NewWebhookSender(smsWebhookURL, smsWebhookToken)
	case "noop":
		smsSender = sms.NewNoopSender()
	default:
		smsSender = sms.NewWebhookSender(smsWebhookURL, smsWebhookToken)
	}

	failSuffix := config.String("NOTIFICATION_FAIL_SUFFIX", "")
	consumerCfg := consumer.Config{
		Brokers: config.String("KAFKA_BROKERS", ""),
		GroupID: config.String("KAFKA_GROUP_ID", "notification-service"),
		Topic:   config.String("KAFKA_CONSUME_TOPIC", "scheduler.reminder.due.v1"),
	}
	eventConsumer := consumer.New(logger, inboxRepo, consumerCfg, func(ctx context.Context, msg kafka.Message) error {
		var payload reminderPayload
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			logger.Error("invalid reminder payload", "err", err)
			return nil
		}
		if payload.AppointmentID == "" || payload.BusinessID == "" || payload.Channel == "" || payload.Recipient == "" || payload.RemindAt == "" {
			logger.Error("missing reminder fields")
			return nil
		}
		if _, err := time.Parse(time.RFC3339, payload.RemindAt); err != nil {
			logger.Error("invalid remind_at", "err", err)
			return nil
		}

		status := "sent"
		failureReason := ""
		if failSuffix != "" && strings.HasSuffix(payload.Recipient, failSuffix) {
			status = "failed"
			failureReason = "simulated failure"
		}

		providerID := ""
		if status == "sent" {
			switch strings.ToLower(payload.Channel) {
			case "email":
				subject := "Appointment reminder"
				body := fmt.Sprintf("Reminder for appointment %s at %s.", payload.AppointmentID, payload.RemindAt)
				if name, ok := payload.TemplateData["business_name"].(string); ok && name != "" {
					body = fmt.Sprintf("[%s] %s", name, body)
				}
				if err := emailSender.Send(payload.Recipient, subject, body); err != nil {
					status = "failed"
					failureReason = err.Error()
					logger.Error("email send failed", "err", err, "recipient", payload.Recipient)
				} else {
					providerID = emailProviderID
				}
			case "sms":
				body := fmt.Sprintf("Reminder: appointment %s at %s.", payload.AppointmentID, payload.RemindAt)
				if name, ok := payload.TemplateData["business_name"].(string); ok && name != "" {
					body = fmt.Sprintf("[%s] %s", name, body)
				}
				if err := smsSender.Send(ctx, payload.Recipient, body); err != nil {
					status = "failed"
					failureReason = err.Error()
					logger.Error("sms send failed", "err", err, "recipient", payload.Recipient)
				} else {
					providerID = smsSender.ProviderID()
				}
			default:
				status = "failed"
				failureReason = "unsupported channel: " + payload.Channel
				logger.Error("unsupported channel", "channel", payload.Channel)
			}
		}

		if err := notificationsRepo.Insert(ctx, storage.Notification{
			AppointmentID: payload.AppointmentID,
			BusinessID:    payload.BusinessID,
			Channel:       payload.Channel,
			Recipient:     payload.Recipient,
			Payload:       payload.TemplateData,
			Status:        status,
		}); err != nil {
			logger.Error("failed to persist notification", "err", err)
			return err
		}

		if status == "failed" {
			if err := writeOutboxFailed(ctx, pool, outboxRepo, payload, failureReason); err != nil {
				logger.Error("failed to enqueue notification.failed", "err", err)
				return err
			}
		} else {
			if err := writeOutboxSent(ctx, pool, outboxRepo, payload, providerID); err != nil {
				logger.Error("failed to enqueue notification.sent", "err", err)
				return err
			}
		}

		logger.Info("reminder processed", "appointment_id", payload.AppointmentID, "channel", payload.Channel, "status", status)
		return nil
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
	handler = otelhttp.NewHandler(handler, "notification")
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
