package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/model"
)

type BookingRepository struct {
	pool *db.Pool
}

type IdempotencyRecord struct {
	BusinessID      string
	IdempotencyKey  string
	AppointmentID   string
	StatusCode      int
	ResponsePayload []byte
}

func NewBookingRepository(pool *db.Pool) *BookingRepository {
	return &BookingRepository{pool: pool}
}

func (r *BookingRepository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func (r *BookingRepository) LockIdempotencyKey(ctx context.Context, tx pgx.Tx, businessID, key string) (IdempotencyRecord, bool, error) {
	rec, err := r.selectIdempotencyForUpdate(ctx, tx, businessID, key)
	if err == nil {
		return rec, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return IdempotencyRecord{}, false, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO booking_idempotency_keys (business_id, idempotency_key)
		VALUES ($1, $2)
		ON CONFLICT (business_id, idempotency_key) DO NOTHING
	`, businessID, key)
	if err != nil {
		return IdempotencyRecord{}, false, err
	}

	rec, err = r.selectIdempotencyForUpdate(ctx, tx, businessID, key)
	if err != nil {
		return IdempotencyRecord{}, false, err
	}
	return rec, false, nil
}

func (r *BookingRepository) FinalizeIdempotency(ctx context.Context, tx pgx.Tx, businessID, key, appointmentID string, statusCode int, response []byte) error {
	_, err := tx.Exec(ctx, `
		UPDATE booking_idempotency_keys
		SET appointment_id = $3,
			status_code = $4,
			response_payload = $5,
			updated_at = now()
		WHERE business_id = $1 AND idempotency_key = $2
	`, businessID, key, appointmentID, statusCode, response)
	return err
}

func (r *BookingRepository) Create(ctx context.Context, tx pgx.Tx, appt *model.Appointment) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO appointments
			(business_id, service_id, staff_id, customer_name, customer_email, customer_phone, start_time, end_time, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, appt.BusinessID, appt.ServiceID, appt.StaffID, appt.CustomerName, appt.CustomerEmail, appt.CustomerPhone,
		appt.StartTime, appt.EndTime, appt.Status).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *BookingRepository) GetAppointmentForUpdate(ctx context.Context, tx pgx.Tx, businessID, appointmentID string) (model.Appointment, error) {
	var appt model.Appointment
	var cancelledAt *time.Time
	err := tx.QueryRow(ctx, `
		SELECT id, business_id, service_id, staff_id, customer_name, customer_email, customer_phone,
			start_time, end_time, status, cancelled_at, COALESCE(cancellation_reason, ''), created_at
		FROM appointments
		WHERE id = $1 AND business_id = $2
		FOR UPDATE
	`, appointmentID, businessID).Scan(
		&appt.ID,
		&appt.BusinessID,
		&appt.ServiceID,
		&appt.StaffID,
		&appt.CustomerName,
		&appt.CustomerEmail,
		&appt.CustomerPhone,
		&appt.StartTime,
		&appt.EndTime,
		&appt.Status,
		&cancelledAt,
		&appt.CancelReason,
		&appt.CreatedAt,
	)
	if err != nil {
		return model.Appointment{}, err
	}
	appt.CancelledAt = cancelledAt
	return appt, nil
}

func (r *BookingRepository) CancelAppointment(ctx context.Context, tx pgx.Tx, businessID, appointmentID, reason string) (time.Time, error) {
	var cancelledAt time.Time
	err := tx.QueryRow(ctx, `
		UPDATE appointments
		SET status = 'cancelled',
			cancelled_at = now(),
			cancellation_reason = $3
		WHERE id = $1 AND business_id = $2
		RETURNING cancelled_at
	`, appointmentID, businessID, reason).Scan(&cancelledAt)
	return cancelledAt, err
}

func (r *BookingRepository) ListBookedIntervals(ctx context.Context, businessID, staffID string, start, end time.Time) ([]model.Appointment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, business_id, service_id, staff_id, customer_name, customer_email, customer_phone,
			start_time, end_time, status, cancelled_at, COALESCE(cancellation_reason, ''), created_at
		FROM appointments
		WHERE business_id = $1
			AND staff_id = $2
			AND status = 'booked'
			AND start_time < $4
			AND end_time > $3
		ORDER BY start_time ASC
	`, businessID, staffID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var appts []model.Appointment
	for rows.Next() {
		var appt model.Appointment
		var cancelledAt *time.Time
		if err := rows.Scan(
			&appt.ID,
			&appt.BusinessID,
			&appt.ServiceID,
			&appt.StaffID,
			&appt.CustomerName,
			&appt.CustomerEmail,
			&appt.CustomerPhone,
			&appt.StartTime,
			&appt.EndTime,
			&appt.Status,
			&cancelledAt,
			&appt.CancelReason,
			&appt.CreatedAt,
		); err != nil {
			return nil, err
		}
		appt.CancelledAt = cancelledAt
		appts = append(appts, appt)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return appts, nil
}

func (r *BookingRepository) ListByBusiness(ctx context.Context, businessID string, limit int) ([]model.Appointment, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, business_id, service_id, staff_id, customer_name, customer_email, customer_phone,
			start_time, end_time, status, cancelled_at, COALESCE(cancellation_reason, ''), created_at
		FROM appointments
		WHERE business_id = $1
		ORDER BY start_time DESC
		LIMIT $2
	`, businessID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var appts []model.Appointment
	for rows.Next() {
		var appt model.Appointment
		var cancelledAt *time.Time
		if err := rows.Scan(
			&appt.ID,
			&appt.BusinessID,
			&appt.ServiceID,
			&appt.StaffID,
			&appt.CustomerName,
			&appt.CustomerEmail,
			&appt.CustomerPhone,
			&appt.StartTime,
			&appt.EndTime,
			&appt.Status,
			&cancelledAt,
			&appt.CancelReason,
			&appt.CreatedAt,
		); err != nil {
			return nil, err
		}
		appt.CancelledAt = cancelledAt
		appts = append(appts, appt)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return appts, nil
}

func IsConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23P01"
}

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func (r *BookingRepository) selectIdempotencyForUpdate(ctx context.Context, tx pgx.Tx, businessID, key string) (IdempotencyRecord, error) {
	var rec IdempotencyRecord
	var responseText string
	err := tx.QueryRow(ctx, `
		SELECT business_id::text,
			idempotency_key,
			COALESCE(appointment_id::text, ''),
			COALESCE(status_code, 0),
			COALESCE(response_payload::text, '')
		FROM booking_idempotency_keys
		WHERE business_id = $1 AND idempotency_key = $2
		FOR UPDATE
	`, businessID, key).Scan(
		&rec.BusinessID,
		&rec.IdempotencyKey,
		&rec.AppointmentID,
		&rec.StatusCode,
		&responseText,
	)
	if err != nil {
		return IdempotencyRecord{}, err
	}
	if responseText != "" {
		rec.ResponsePayload = []byte(responseText)
	}
	return rec, nil
}
