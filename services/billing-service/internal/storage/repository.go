package storage

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/md-rashed-zaman/apptremind/libs/db"
)

type Repository struct {
	pool *db.Pool
}

func NewRepository(pool *db.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

type Subscription struct {
	BusinessID           string
	Tier                 string
	Status               string
	Provider             string
	StripeCustomerID     string
	StripeSubscriptionID string
	CurrentPeriodStart   *time.Time
	CurrentPeriodEnd     *time.Time
	UpdatedAt            time.Time
}

func (r *Repository) UpsertSubscription(ctx context.Context, tx pgx.Tx, s Subscription) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO subscriptions (business_id, tier, status, provider, stripe_customer_id, stripe_subscription_id, current_period_start, current_period_end)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (business_id)
		DO UPDATE SET tier = EXCLUDED.tier,
		              status = EXCLUDED.status,
		              provider = EXCLUDED.provider,
		              stripe_customer_id = EXCLUDED.stripe_customer_id,
		              stripe_subscription_id = EXCLUDED.stripe_subscription_id,
		              current_period_start = EXCLUDED.current_period_start,
		              current_period_end = EXCLUDED.current_period_end,
		              updated_at = now()
	`, s.BusinessID, s.Tier, s.Status, defaultIfEmpty(s.Provider, "local"), nullIfEmpty(s.StripeCustomerID), nullIfEmpty(s.StripeSubscriptionID), s.CurrentPeriodStart, s.CurrentPeriodEnd)
	return err
}

func (r *Repository) GetSubscription(ctx context.Context, businessID string) (Subscription, error) {
	var s Subscription
	var cps *time.Time
	var cpe *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT business_id::text, tier, status, provider,
		       COALESCE(stripe_customer_id, ''), COALESCE(stripe_subscription_id, ''),
		       current_period_start, current_period_end, updated_at
		FROM subscriptions
		WHERE business_id = $1
	`, businessID).Scan(&s.BusinessID, &s.Tier, &s.Status, &s.Provider, &s.StripeCustomerID, &s.StripeSubscriptionID, &cps, &cpe, &s.UpdatedAt)
	if err != nil {
		return Subscription{}, err
	}
	s.CurrentPeriodStart = cps
	s.CurrentPeriodEnd = cpe
	return s, nil
}

func (r *Repository) GetSubscriptionForUpdate(ctx context.Context, tx pgx.Tx, businessID string) (Subscription, bool, error) {
	var s Subscription
	var cps *time.Time
	var cpe *time.Time
	err := tx.QueryRow(ctx, `
		SELECT business_id::text, tier, status, provider,
		       COALESCE(stripe_customer_id, ''), COALESCE(stripe_subscription_id, ''),
		       current_period_start, current_period_end, updated_at
		FROM subscriptions
		WHERE business_id = $1
		FOR UPDATE
	`, businessID).Scan(&s.BusinessID, &s.Tier, &s.Status, &s.Provider, &s.StripeCustomerID, &s.StripeSubscriptionID, &cps, &cpe, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Subscription{}, false, nil
		}
		return Subscription{}, false, err
	}
	s.CurrentPeriodStart = cps
	s.CurrentPeriodEnd = cpe
	return s, true, nil
}

func (r *Repository) ListStripeSubscriptionsForReconcile(ctx context.Context, limit int) ([]Subscription, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT business_id::text, tier, status, provider,
		       COALESCE(stripe_customer_id, ''), COALESCE(stripe_subscription_id, ''),
		       current_period_start, current_period_end, updated_at
		FROM subscriptions
		WHERE provider = 'stripe' AND stripe_subscription_id IS NOT NULL AND stripe_subscription_id <> ''
		ORDER BY updated_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Subscription
	for rows.Next() {
		var s Subscription
		var cps *time.Time
		var cpe *time.Time
		if err := rows.Scan(&s.BusinessID, &s.Tier, &s.Status, &s.Provider, &s.StripeCustomerID, &s.StripeSubscriptionID, &cps, &cpe, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.CurrentPeriodStart = cps
		s.CurrentPeriodEnd = cpe
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type CheckoutSession struct {
	StripeSessionID      string
	BusinessID           string
	Tier                 string
	Status               string
	StripeCustomerID     string
	StripeSubscriptionID string
	URL                  string
	ReturnToken          string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	CompletedAt          *time.Time
	CanceledAt           *time.Time
	ReturnSeenAt         *time.Time
	ExpiredAt            *time.Time
}

func (r *Repository) UpsertCheckoutSession(ctx context.Context, tx pgx.Tx, s CheckoutSession) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO checkout_sessions (stripe_session_id, business_id, tier, status, url, return_token)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (stripe_session_id)
		DO UPDATE SET business_id = EXCLUDED.business_id,
		              tier = EXCLUDED.tier,
		              status = EXCLUDED.status,
		              url = EXCLUDED.url,
		              updated_at = now()
	`, s.StripeSessionID, s.BusinessID, s.Tier, s.Status, nullIfEmpty(s.URL), nullIfEmpty(s.ReturnToken))
	return err
}

func (r *Repository) MarkCheckoutSessionCompleted(ctx context.Context, tx pgx.Tx, stripeSessionID string, completedAt time.Time, stripeCustomerID, stripeSubscriptionID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE checkout_sessions
		SET status = 'completed',
		    stripe_customer_id = $3,
		    stripe_subscription_id = $4,
		    completed_at = $2,
		    updated_at = now()
		WHERE stripe_session_id = $1
	`, stripeSessionID, completedAt, nullIfEmpty(stripeCustomerID), nullIfEmpty(stripeSubscriptionID))
	return err
}

func (r *Repository) MarkCheckoutSessionCanceled(ctx context.Context, tx pgx.Tx, stripeSessionID string, canceledAt time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE checkout_sessions
		SET status = 'canceled',
		    canceled_at = $2,
		    updated_at = now()
		WHERE stripe_session_id = $1
	`, stripeSessionID, canceledAt)
	return err
}

func (r *Repository) MarkCheckoutSessionExpired(ctx context.Context, tx pgx.Tx, stripeSessionID string, expiredAt time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE checkout_sessions
		SET status = 'expired',
		    expired_at = $2,
		    updated_at = now()
		WHERE stripe_session_id = $1 AND status <> 'completed'
	`, stripeSessionID, expiredAt)
	return err
}

func (r *Repository) AckCheckoutReturn(ctx context.Context, tx pgx.Tx, stripeSessionID string, token string, result string, seenAt time.Time) error {
	// Token protects this public endpoint from being used to tamper with other sessions.
	// Only mark canceled if it wasn't already completed (Stripe webhook is the source of truth).
	if strings.TrimSpace(result) == "" {
		result = "unknown"
	}
	_, err := tx.Exec(ctx, `
		UPDATE checkout_sessions
		SET return_seen_at = $4,
		    status = CASE
		      WHEN $3 = 'cancel' AND status <> 'completed' THEN 'canceled'
		      ELSE status
		    END,
		    canceled_at = CASE
		      WHEN $3 = 'cancel' AND status <> 'completed' THEN COALESCE(canceled_at, $4)
		      ELSE canceled_at
		    END,
		    updated_at = now()
		WHERE stripe_session_id = $1 AND return_token = $2
	`, stripeSessionID, token, result, seenAt)
	return err
}

func (r *Repository) GetCheckoutSession(ctx context.Context, stripeSessionID string) (CheckoutSession, error) {
	var s CheckoutSession
	var completedAt *time.Time
	var canceledAt *time.Time
	var returnSeenAt *time.Time
	var expiredAt *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT stripe_session_id, business_id::text, tier, status,
		       COALESCE(stripe_customer_id, ''), COALESCE(stripe_subscription_id, ''),
		       COALESCE(url, ''), COALESCE(return_token, ''), created_at, updated_at, completed_at, canceled_at, return_seen_at, expired_at
		FROM checkout_sessions
		WHERE stripe_session_id = $1
	`, stripeSessionID).Scan(
		&s.StripeSessionID,
		&s.BusinessID,
		&s.Tier,
		&s.Status,
		&s.StripeCustomerID,
		&s.StripeSubscriptionID,
		&s.URL,
		&s.ReturnToken,
		&s.CreatedAt,
		&s.UpdatedAt,
		&completedAt,
		&canceledAt,
		&returnSeenAt,
		&expiredAt,
	)
	if err != nil {
		return CheckoutSession{}, err
	}
	s.CompletedAt = completedAt
	s.CanceledAt = canceledAt
	s.ReturnSeenAt = returnSeenAt
	s.ExpiredAt = expiredAt
	return s, nil
}

type ProviderEvent struct {
	Provider        string
	ProviderEventID string
	EventType       string
	Payload         []byte
}

var ErrDuplicateProviderEvent = errors.New("duplicate provider event")

func (r *Repository) InsertProviderEvent(ctx context.Context, tx pgx.Tx, evt ProviderEvent) error {
	var payload any
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		// keep raw JSON error as a hard failure: webhook should be well-formed.
		return err
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO provider_events (provider, provider_event_id, event_type, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider, provider_event_id) DO NOTHING
	`, evt.Provider, evt.ProviderEventID, evt.EventType, payload)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDuplicateProviderEvent
	}
	return nil
}

type AuditEvent struct {
	EventType  string
	ActorType  string
	ActorID    string
	BusinessID string
	Metadata   []byte
}

func (r *Repository) InsertAuditEvent(ctx context.Context, tx pgx.Tx, evt AuditEvent) error {
	var payload any
	if len(evt.Metadata) == 0 {
		payload = map[string]any{}
	} else if err := json.Unmarshal(evt.Metadata, &payload); err != nil {
		return err
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO audit_events (event_type, actor_type, actor_id, business_id, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`, evt.EventType, evt.ActorType, nullIfEmpty(evt.ActorID), nullIfEmpty(evt.BusinessID), payload)
	return err
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func defaultIfEmpty(s string, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
