package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/services/auth-service/internal/outbox"
)

type Repository struct {
	pool *db.Pool
}

func NewRepository(pool *db.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Record(ctx context.Context, eventType string, actorID string, metadata map[string]any) error {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO audit_events (event_type, actor_id, metadata)
		VALUES ($1, NULLIF($2, ''), $3)
	`, eventType, actorID, raw)
	return err
}

func (r *Repository) RecordWithOutbox(ctx context.Context, outboxRepo *outbox.Repository, eventType string, actorID string, metadata map[string]any) error {
	if outboxRepo == nil {
		return r.Record(ctx, eventType, actorID, metadata)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_events (event_type, actor_id, metadata)
		VALUES ($1, NULLIF($2, ''), $3)
	`, eventType, actorID, raw)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{
		"event_type": eventType,
		"actor_id":   actorID,
		"metadata":   metadata,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	if err := outboxRepo.Insert(ctx, tx, outbox.Event{
		AggregateType: "audit_event",
		AggregateID:   "auth",
		EventType:     "auth.audit.v1",
		Payload:       payload,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

type AuditEvent struct {
	ID        int64           `json:"id"`
	EventType string          `json:"event_type"`
	ActorID   string          `json:"actor_id,omitempty"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt string          `json:"created_at"`
}

func (r *Repository) ListRecent(ctx context.Context, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_type, COALESCE(actor_id::text, ''), metadata, created_at
		FROM audit_events
		ORDER BY id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var createdAt time.Time
		if err := rows.Scan(&e.ID, &e.EventType, &e.ActorID, &e.Metadata, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		events = append(events, e)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return events, nil
}
