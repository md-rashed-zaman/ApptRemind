package consumer

import (
	"context"
	"log/slog"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/kafkax"
	"github.com/md-rashed-zaman/apptremind/services/scheduler-service/internal/inbox"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Handler func(ctx context.Context, msg kafka.Message) error

type Consumer struct {
	reader  *kafka.Reader
	logger  *slog.Logger
	inbox   *inbox.Repository
	handler Handler
}

type Config struct {
	Brokers string
	GroupID string
	Topic   string
}

func New(logger *slog.Logger, inboxRepo *inbox.Repository, cfg Config, handler Handler) *Consumer {
	brokers := kafkax.SplitBrokers(cfg.Brokers)
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  cfg.GroupID,
		Topic:    cfg.Topic,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	return &Consumer{
		reader:  reader,
		logger:  logger,
		inbox:   inboxRepo,
		handler: handler,
	}
}

func (c *Consumer) Run(ctx context.Context) {
	defer c.reader.Close()

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logger.Error("kafka read error", "err", err)
			time.Sleep(1 * time.Second)
			continue
		}

		ctxMsg := kafkax.ExtractTraceContext(ctx, msg)
		ctxSpan, span := otel.Tracer("kafka").Start(ctxMsg, "kafka.consume",
			trace.WithAttributes(
				attribute.String("messaging.system", "kafka"),
				attribute.String("messaging.destination", msg.Topic),
			),
		)

		meta := kafkax.ExtractEventMeta(msg)

		ok, err := c.inbox.Record(ctxSpan, meta.EventID, meta.EventType)
		if err != nil {
			c.logger.Error("inbox record failed", "err", err)
			span.RecordError(err)
			span.End()
			continue
		}
		if !ok {
			c.logger.Info("duplicate event ignored", "event_id", meta.EventID, "event_type", meta.EventType)
			span.End()
			continue
		}

		if err := c.handler(ctxSpan, msg); err != nil {
			c.logger.Error("handler error", "err", err, "event_id", meta.EventID)
			span.RecordError(err)
			span.End()
			continue
		}
		span.End()
	}
}
