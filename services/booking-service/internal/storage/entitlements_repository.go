package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type BusinessEntitlements struct {
	BusinessID            string
	Tier                  string
	MaxMonthlyAppointments int
	UpdatedAt             time.Time
}

func (r *BookingRepository) UpsertBusinessEntitlements(ctx context.Context, tx pgx.Tx, ent BusinessEntitlements) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO business_entitlements (business_id, tier, max_monthly_appointments)
		VALUES ($1, $2, $3)
		ON CONFLICT (business_id)
		DO UPDATE SET tier = EXCLUDED.tier,
		              max_monthly_appointments = EXCLUDED.max_monthly_appointments,
		              updated_at = now()
	`, ent.BusinessID, ent.Tier, ent.MaxMonthlyAppointments)
	return err
}

func (r *BookingRepository) GetBusinessEntitlements(ctx context.Context, tx pgx.Tx, businessID string) (BusinessEntitlements, bool, error) {
	var ent BusinessEntitlements
	err := tx.QueryRow(ctx, `
		SELECT business_id::text, tier, max_monthly_appointments, updated_at
		FROM business_entitlements
		WHERE business_id = $1
	`, businessID).Scan(&ent.BusinessID, &ent.Tier, &ent.MaxMonthlyAppointments, &ent.UpdatedAt)
	if err != nil {
		if IsNotFound(err) {
			return BusinessEntitlements{}, false, nil
		}
		return BusinessEntitlements{}, false, err
	}
	return ent, true, nil
}

func (r *BookingRepository) CountBookedByBusinessInRange(ctx context.Context, tx pgx.Tx, businessID string, startInclusive, endExclusive time.Time) (int, error) {
	var cnt int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM appointments
		WHERE business_id = $1
		  AND status = 'booked'
		  AND start_time >= $2
		  AND start_time < $3
	`, businessID, startInclusive, endExclusive).Scan(&cnt)
	return cnt, err
}

