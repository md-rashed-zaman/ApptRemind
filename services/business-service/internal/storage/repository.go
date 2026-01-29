package storage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/md-rashed-zaman/apptremind/libs/db"
)

type Repository struct {
	pool *db.Pool
}

func NewRepository(pool *db.Pool) *Repository {
	return &Repository{pool: pool}
}

type BusinessProfile struct {
	BusinessID  string
	Name        string
	Timezone    string
	OffsetsMins []int
}

func (r *Repository) GetOrCreateProfile(ctx context.Context, businessID string) (BusinessProfile, error) {
	// Create a default profile if missing (keeps dev UX smooth while other services mature).
	_, err := r.pool.Exec(ctx, `
		INSERT INTO business_profiles (business_id)
		VALUES ($1)
		ON CONFLICT (business_id) DO NOTHING
	`, businessID)
	if err != nil {
		return BusinessProfile{}, err
	}

	var p BusinessProfile
	err = r.pool.QueryRow(ctx, `
		SELECT business_id::text, name, timezone, reminder_offsets_minutes
		FROM business_profiles
		WHERE business_id = $1
	`, businessID).Scan(&p.BusinessID, &p.Name, &p.Timezone, &p.OffsetsMins)
	return p, err
}

func (r *Repository) UpdateProfile(ctx context.Context, businessID string, name string, timezone string, offsetsMins []int) error {
	if len(offsetsMins) == 0 {
		offsetsMins = []int{1440, 60}
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO business_profiles (business_id, name, timezone, reminder_offsets_minutes)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (business_id) DO UPDATE
		SET name = EXCLUDED.name,
			timezone = EXCLUDED.timezone,
			reminder_offsets_minutes = EXCLUDED.reminder_offsets_minutes,
			updated_at = now()
	`, businessID, name, timezone, offsetsMins)
	return err
}

type BusinessService struct {
	ID           string
	BusinessID   string
	Name         string
	DurationMins int
	Price        string
	Description  string
	CreatedAt    time.Time
}

func (r *Repository) CreateService(ctx context.Context, businessID, name string, durationMinutes int, price string, description string) (string, error) {
	id := uuid.NewString()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO business_services (id, business_id, name, duration_minutes, price, description)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, businessID, name, durationMinutes, price, description)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *Repository) ListServices(ctx context.Context, businessID string, limit int) ([]BusinessService, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, business_id::text, name, duration_minutes, price::text, description, created_at
		FROM business_services
		WHERE business_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, businessID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BusinessService
	for rows.Next() {
		var s BusinessService
		if err := rows.Scan(&s.ID, &s.BusinessID, &s.Name, &s.DurationMins, &s.Price, &s.Description, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

func (r *Repository) GetServiceDuration(ctx context.Context, businessID, serviceID string) (int, error) {
	var mins int
	err := r.pool.QueryRow(ctx, `
		SELECT duration_minutes
		FROM business_services
		WHERE business_id = $1 AND id = $2
	`, businessID, serviceID).Scan(&mins)
	return mins, err
}

type Staff struct {
	ID         string
	BusinessID string
	Name       string
	IsActive   bool
}

func (r *Repository) CreateStaff(ctx context.Context, businessID, name string, isActive bool) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO staff (business_id, name, is_active)
		VALUES ($1, $2, $3)
		RETURNING id::text
	`, businessID, name, isActive).Scan(&id)
	if err != nil {
		return "", err
	}

	// Default schedule: Mon-Fri 09:00-17:00 working, Sat/Sun closed.
	for wd := 0; wd <= 6; wd++ {
		isWorking := wd >= 1 && wd <= 5
		startMin := 540
		endMin := 1020
		if !isWorking {
			startMin = 0
			endMin = 0
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO staff_working_hours (staff_id, weekday, is_working, start_minute, end_minute)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (staff_id, weekday) DO NOTHING
		`, id, wd, isWorking, startMin, endMin); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

func (r *Repository) ListStaff(ctx context.Context, businessID string, limit int) ([]Staff, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, business_id::text, name, is_active
		FROM staff
		WHERE business_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, businessID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Staff
	for rows.Next() {
		var s Staff
		if err := rows.Scan(&s.ID, &s.BusinessID, &s.Name, &s.IsActive); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

type WorkingHours struct {
	StaffID     string
	Weekday     int
	IsWorking   bool
	StartMinute int
	EndMinute   int
}

func (r *Repository) GetWorkingHours(ctx context.Context, businessID, staffID string, weekday int) (WorkingHours, error) {
	var wh WorkingHours
	err := r.pool.QueryRow(ctx, `
		SELECT h.staff_id::text, h.weekday, h.is_working, h.start_minute, h.end_minute
		FROM staff_working_hours h
		JOIN staff s ON s.id = h.staff_id
		WHERE s.business_id = $1 AND h.staff_id = $2 AND h.weekday = $3
	`, businessID, staffID, weekday).Scan(&wh.StaffID, &wh.Weekday, &wh.IsWorking, &wh.StartMinute, &wh.EndMinute)
	if err == nil {
		return wh, nil
	}
	if err == pgx.ErrNoRows {
		// Default fallback if schedule wasn't seeded.
		return WorkingHours{StaffID: staffID, Weekday: weekday, IsWorking: weekday >= 1 && weekday <= 5, StartMinute: 540, EndMinute: 1020}, nil
	}
	return WorkingHours{}, err
}

func (r *Repository) ListWorkingHours(ctx context.Context, businessID, staffID string) ([]WorkingHours, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT h.staff_id::text, h.weekday, h.is_working, h.start_minute, h.end_minute
		FROM staff_working_hours h
		JOIN staff s ON s.id = h.staff_id
		WHERE s.business_id = $1 AND h.staff_id = $2
		ORDER BY h.weekday ASC
	`, businessID, staffID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorkingHours
	for rows.Next() {
		var wh WorkingHours
		if err := rows.Scan(&wh.StaffID, &wh.Weekday, &wh.IsWorking, &wh.StartMinute, &wh.EndMinute); err != nil {
			return nil, err
		}
		out = append(out, wh)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

func (r *Repository) UpsertWorkingHours(ctx context.Context, businessID, staffID string, weekday int, isWorking bool, startMinute int, endMinute int) error {
	var exists bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM staff WHERE id = $1 AND business_id = $2
		)
	`, staffID, businessID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO staff_working_hours (staff_id, weekday, is_working, start_minute, end_minute)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (staff_id, weekday) DO UPDATE
		SET is_working = EXCLUDED.is_working,
			start_minute = EXCLUDED.start_minute,
			end_minute = EXCLUDED.end_minute
	`, staffID, weekday, isWorking, startMinute, endMinute)
	return err
}

type TimeOff struct {
	ID        string
	StaffID   string
	StartTime time.Time
	EndTime   time.Time
	Reason    string
	CreatedAt time.Time
}

func (r *Repository) CreateTimeOff(ctx context.Context, businessID, staffID string, startTime, endTime time.Time, reason string) (string, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM staff WHERE id = $1 AND business_id = $2
		)
	`, staffID, businessID).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		return "", pgx.ErrNoRows
	}

	id := uuid.NewString()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO staff_time_off (id, staff_id, start_time, end_time, reason)
		VALUES ($1, $2, $3, $4, $5)
	`, id, staffID, startTime, endTime, reason)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *Repository) ListTimeOff(ctx context.Context, businessID, staffID string, from, to time.Time, limit int) ([]TimeOff, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT t.id::text, t.staff_id::text, t.start_time, t.end_time, t.reason, t.created_at
		FROM staff_time_off t
		JOIN staff s ON s.id = t.staff_id
		WHERE s.business_id = $1
			AND t.staff_id = $2
			AND t.end_time > $3
			AND t.start_time < $4
		ORDER BY t.start_time ASC
		LIMIT $5
	`, businessID, staffID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TimeOff
	for rows.Next() {
		var t TimeOff
		if err := rows.Scan(&t.ID, &t.StaffID, &t.StartTime, &t.EndTime, &t.Reason, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

func (r *Repository) DeleteTimeOff(ctx context.Context, businessID, timeOffID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM staff_time_off t
		USING staff s
		WHERE t.staff_id = s.id
		  AND s.business_id = $1
		  AND t.id = $2
	`, businessID, timeOffID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
