package jobs

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/md-rashed-zaman/apptremind/libs/db"
	otelx "github.com/md-rashed-zaman/apptremind/libs/otel"
	"github.com/md-rashed-zaman/apptremind/services/scheduler-service/internal/outbox"
)

type Worker struct {
	pool      *db.Pool
	repo      *Repository
	outbox    *outbox.Repository
	logger    *slog.Logger
	interval  time.Duration
	batchSize int
	backoff   time.Duration
}

type WorkerConfig struct {
	Interval  time.Duration
	BatchSize int
	Backoff   time.Duration
}

func NewWorker(pool *db.Pool, repo *Repository, outboxRepo *outbox.Repository, logger *slog.Logger, cfg WorkerConfig) *Worker {
	if cfg.Interval <= 0 {
		cfg.Interval = 2 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	if cfg.Backoff <= 0 {
		cfg.Backoff = 1 * time.Minute
	}
	return &Worker{
		pool:      pool,
		repo:      repo,
		outbox:    outboxRepo,
		logger:    logger,
		interval:  cfg.Interval,
		batchSize: cfg.BatchSize,
		backoff:   cfg.Backoff,
	}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.processBatch(ctx); err != nil {
				w.logger.Error("scheduler batch failed", "err", err)
			}
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	jobs, err := w.repo.FetchDue(ctx, tx, w.batchSize)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return tx.Commit(ctx)
	}

	var ids []int64
	var failed []Job
	for _, job := range jobs {
		jobCtx := otelx.ContextWithTraceContext(ctx, job.Traceparent, job.Tracestate)
		payload, err := json.Marshal(map[string]any{
			"appointment_id": job.AppointmentID,
			"business_id":    job.BusinessID,
			"channel":        job.Channel,
			"recipient":      job.Recipient,
			"remind_at":      job.RemindAt.UTC().Format(time.RFC3339),
			"template_data":  job.TemplateData,
		})
		if err != nil {
			failed = append(failed, job)
			continue
		}

		if err := w.outbox.Insert(jobCtx, tx, outbox.Event{
			AggregateType: "scheduler_job",
			AggregateID:   job.AppointmentID,
			EventType:     "scheduler.reminder.due.v1",
			Payload:       payload,
		}); err != nil {
			failed = append(failed, job)
			continue
		}
		ids = append(ids, job.ID)
	}

	if err := w.repo.MarkProcessed(ctx, tx, ids); err != nil {
		return err
	}

	for _, job := range failed {
		jobCtx := otelx.ContextWithTraceContext(ctx, job.Traceparent, job.Tracestate)
		nextRunAt := time.Now().UTC().Add(w.backoff)
		attempts := job.Attempts + 1
		if err := w.repo.MarkFailed(ctx, tx, job.ID, attempts, job.MaxAttempts, nextRunAt, "outbox enqueue failed"); err != nil {
			return err
		}

		if attempts >= job.MaxAttempts {
			if err := w.enqueueDLQ(jobCtx, tx, job, "max attempts reached"); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func (w *Worker) enqueueDLQ(ctx context.Context, tx pgx.Tx, job Job, reason string) error {
	payload, err := json.Marshal(map[string]any{
		"appointment_id": job.AppointmentID,
		"business_id":    job.BusinessID,
		"channel":        job.Channel,
		"recipient":      job.Recipient,
		"remind_at":      job.RemindAt.UTC().Format(time.RFC3339),
		"template_data":  job.TemplateData,
		"error_reason":   reason,
		"failed_at":      time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	return w.outbox.Insert(ctx, tx, outbox.Event{
		AggregateType: "scheduler_job",
		AggregateID:   job.AppointmentID,
		EventType:     "scheduler.reminder.dlq.v1",
		Payload:       payload,
	})
}
