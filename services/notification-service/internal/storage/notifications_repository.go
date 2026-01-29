package storage

import (
	"context"
	"encoding/json"

	"github.com/md-rashed-zaman/apptremind/libs/db"
)

type Notification struct {
	AppointmentID string
	BusinessID    string
	Channel       string
	Recipient     string
	Payload       map[string]any
	Status        string
}

type Repository struct {
	pool *db.Pool
}

func NewRepository(pool *db.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Insert(ctx context.Context, n Notification) error {
	payload, err := json.Marshal(n.Payload)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO notifications (appointment_id, business_id, channel, recipient, payload, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, n.AppointmentID, n.BusinessID, n.Channel, n.Recipient, payload, n.Status)
	return err
}

