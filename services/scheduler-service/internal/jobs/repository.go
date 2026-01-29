package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	otelx "github.com/md-rashed-zaman/apptremind/libs/otel"
)

type Job struct {
	ID             int64
	IdempotencyKey string
	AppointmentID  string
	BusinessID     string
	Channel        string
	Recipient      string
	RemindAt       time.Time
	TemplateData   map[string]any
	Traceparent    string
	Tracestate     string
	Attempts       int
	MaxAttempts    int
	NextRunAt      time.Time
}

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) Insert(ctx context.Context, tx pgx.Tx, job Job) error {
	payload, err := json.Marshal(job.TemplateData)
	if err != nil {
		return err
	}
	traceparent, tracestate := otelx.TraceContextStrings(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO scheduler_jobs (idempotency_key, appointment_id, business_id, channel, recipient, remind_at, template_data, next_run_at, traceparent, tracestate)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $6, $8, $9)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, job.IdempotencyKey, job.AppointmentID, job.BusinessID, job.Channel, job.Recipient, job.RemindAt, payload, traceparent, tracestate)
	return err
}

func (r *Repository) FetchDue(ctx context.Context, tx pgx.Tx, limit int) ([]Job, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, idempotency_key, appointment_id, business_id, channel, recipient, remind_at, template_data, traceparent, tracestate, attempts, max_attempts, next_run_at
		FROM scheduler_jobs
		WHERE status = 'pending' AND next_run_at <= now()
		ORDER BY next_run_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		var raw []byte
		if err := rows.Scan(&j.ID, &j.IdempotencyKey, &j.AppointmentID, &j.BusinessID, &j.Channel, &j.Recipient, &j.RemindAt, &raw, &j.Traceparent, &j.Tracestate, &j.Attempts, &j.MaxAttempts, &j.NextRunAt); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &j.TemplateData); err != nil {
				return nil, err
			}
		} else {
			j.TemplateData = map[string]any{}
		}
		jobs = append(jobs, j)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return jobs, nil
}

func (r *Repository) MarkProcessed(ctx context.Context, tx pgx.Tx, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		UPDATE scheduler_jobs
		SET status = 'processed', updated_at = now()
		WHERE id = ANY($1)
	`, ids)
	return err
}

func (r *Repository) MarkFailed(ctx context.Context, tx pgx.Tx, id int64, attempts int, maxAttempts int, nextRunAt time.Time, lastError string) error {
	status := "pending"
	if attempts >= maxAttempts {
		status = "failed"
	}
	_, err := tx.Exec(ctx, `
		UPDATE scheduler_jobs
		SET attempts = $2,
		    status = $3,
		    next_run_at = $4,
		    last_error = $5,
		    updated_at = now()
		WHERE id = $1
	`, id, attempts, status, nextRunAt, lastError)
	return err
}
