package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/libs/kafkax"
	otelx "github.com/md-rashed-zaman/apptremind/libs/otel"
	"github.com/segmentio/kafka-go"
)

type Publisher struct {
	pool      *db.Pool
	repo      *Repository
	logger    *slog.Logger
	brokers   []string
	pollEvery time.Duration
	batchSize int
}

type PublisherConfig struct {
	Brokers   string
	PollEvery time.Duration
	BatchSize int
}

func NewPublisher(pool *db.Pool, repo *Repository, logger *slog.Logger, cfg PublisherConfig) *Publisher {
	brokers := kafkax.SplitBrokers(cfg.Brokers)
	if cfg.PollEvery <= 0 {
		cfg.PollEvery = 2 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	return &Publisher{
		pool:      pool,
		repo:      repo,
		logger:    logger,
		brokers:   brokers,
		pollEvery: cfg.PollEvery,
		batchSize: cfg.BatchSize,
	}
}

func (p *Publisher) Run(ctx context.Context) {
	if len(p.brokers) == 0 {
		p.logger.Warn("outbox publisher disabled (no kafka brokers configured)")
		return
	}

	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  p.brokers,
		Balancer: &kafka.Hash{},
	})
	defer writer.Close()

	ticker := time.NewTicker(p.pollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.publishBatch(ctx, writer); err != nil {
				p.logger.Error("outbox publish failed", "err", err)
			}
		}
	}
}

func (p *Publisher) publishBatch(ctx context.Context, writer *kafka.Writer) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	records, err := p.repo.FetchUnpublished(ctx, tx, p.batchSize)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return tx.Commit(ctx)
	}

	for _, r := range records {
		msgCtx := otelx.ContextWithTraceContext(ctx, r.Traceparent, r.Tracestate)
		msg := kafka.Message{
			Topic: r.EventType,
			Key:   []byte(r.AggregateID),
			Value: r.Payload,
			Headers: []kafka.Header{
				{Key: "event_id", Value: []byte(r.EventID)},
				{Key: "event_type", Value: []byte(r.EventType)},
			},
		}
		msg.Headers = kafkax.InjectTraceHeaders(msgCtx, msg.Headers)
		if err := writer.WriteMessages(ctx, msg); err != nil {
			return err
		}
	}

	var ids []int64
	for _, r := range records {
		ids = append(ids, r.ID)
	}
	if err := p.repo.MarkPublished(ctx, tx, ids); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

