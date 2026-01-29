package inbox

import (
	"context"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/md-rashed-zaman/apptremind/libs/db"
)

type Repository struct {
	pool *db.Pool
}

func NewRepository(pool *db.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Record(ctx context.Context, eventID string, eventType string) (bool, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO inbox_events (event_id, event_type)
		VALUES ($1, $2)
	`, eventID, eventType)
	if err == nil {
		return true, nil
	}

	if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
		return false, nil
	}

	return false, err
}
