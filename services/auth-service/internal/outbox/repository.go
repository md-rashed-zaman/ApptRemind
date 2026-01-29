package outbox

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/md-rashed-zaman/apptremind/libs/db"
	otelx "github.com/md-rashed-zaman/apptremind/libs/otel"
)

type Repository struct {
	pool *db.Pool
}

func NewRepository(pool *db.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Insert(ctx context.Context, tx pgx.Tx, evt Event) error {
	traceparent, tracestate := otelx.TraceContextStrings(ctx)
	_, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload, traceparent, tracestate)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, evt.AggregateType, evt.AggregateID, evt.EventType, evt.Payload, traceparent, tracestate)
	return err
}

type Record struct {
	ID            int64
	EventID       string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	Traceparent   string
	Tracestate    string
	CreatedAt     time.Time
}

func (r *Repository) FetchUnpublished(ctx context.Context, tx pgx.Tx, limit int) ([]Record, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, event_id, aggregate_type, aggregate_id, event_type, payload, traceparent, tracestate, created_at
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY id
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var rcd Record
		if err := rows.Scan(&rcd.ID, &rcd.EventID, &rcd.AggregateType, &rcd.AggregateID, &rcd.EventType, &rcd.Payload, &rcd.Traceparent, &rcd.Tracestate, &rcd.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, rcd)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return records, nil
}

func (r *Repository) MarkPublished(ctx context.Context, tx pgx.Tx, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		UPDATE outbox_events
		SET published_at = now()
		WHERE id = ANY($1)
	`, ids)
	return err
}
