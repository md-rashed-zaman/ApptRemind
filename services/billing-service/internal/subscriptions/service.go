package subscriptions

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/entitlements"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/outbox"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/storage"
)

// Service encapsulates subscription state transitions and the side effects (outbox events).
// Keeping this out of HTTP handlers makes it reusable for webhook + reconciliation flows.
type Service struct {
	repo       *storage.Repository
	outboxRepo *outbox.Repository
}

func New(repo *storage.Repository, outboxRepo *outbox.Repository) *Service {
	return &Service{repo: repo, outboxRepo: outboxRepo}
}

func (s *Service) ApplyActivated(ctx context.Context, tx pgx.Tx, businessID, tier string, activatedAt time.Time, provider string, stripeCustomerID string, stripeSubscriptionID string, periodStart, periodEnd *time.Time) error {
	existing, ok, err := s.repo.GetSubscriptionForUpdate(ctx, tx, businessID)
	if err != nil {
		return err
	}

	if err := s.repo.UpsertSubscription(ctx, tx, storage.Subscription{
		BusinessID:           businessID,
		Tier:                 tier,
		Status:               "active",
		Provider:             provider,
		StripeCustomerID:     stripeCustomerID,
		StripeSubscriptionID: stripeSubscriptionID,
		CurrentPeriodStart:   periodStart,
		CurrentPeriodEnd:     periodEnd,
	}); err != nil {
		return err
	}

	// Only emit when the effective entitlement changes (tier/status). Provider ID updates alone shouldn't fan out.
	if ok && existing.Status == "active" && existing.Tier == tier {
		return nil
	}

	limits := entitlements.LimitsForTier(tier)
	payload, err := json.Marshal(map[string]any{
		"business_id":              businessID,
		"tier":                     limits.Tier,
		"max_monthly_appointments": limits.MaxMonthlyAppointments,
		"activated_at":             activatedAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	return s.outboxRepo.Insert(ctx, tx, outbox.Event{
		AggregateType: "subscription",
		AggregateID:   businessID,
		EventType:     "billing.subscription.activated.v1",
		Payload:       payload,
	})
}

func (s *Service) ApplyCanceled(ctx context.Context, tx pgx.Tx, businessID string, canceledAt time.Time, provider string, stripeCustomerID string, stripeSubscriptionID string, periodStart, periodEnd *time.Time) error {
	existing, ok, err := s.repo.GetSubscriptionForUpdate(ctx, tx, businessID)
	if err != nil {
		return err
	}

	if err := s.repo.UpsertSubscription(ctx, tx, storage.Subscription{
		BusinessID:           businessID,
		Tier:                 "free",
		Status:               "canceled",
		Provider:             provider,
		StripeCustomerID:     stripeCustomerID,
		StripeSubscriptionID: stripeSubscriptionID,
		CurrentPeriodStart:   periodStart,
		CurrentPeriodEnd:     periodEnd,
	}); err != nil {
		return err
	}

	// Only emit when the effective entitlement changes (tier/status).
	if ok && existing.Status == "canceled" && existing.Tier == "free" {
		return nil
	}

	limits := entitlements.LimitsForTier("free")
	payload, err := json.Marshal(map[string]any{
		"business_id":              businessID,
		"tier":                     limits.Tier,
		"max_monthly_appointments": limits.MaxMonthlyAppointments,
		"canceled_at":              canceledAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	return s.outboxRepo.Insert(ctx, tx, outbox.Event{
		AggregateType: "subscription",
		AggregateID:   businessID,
		EventType:     "billing.subscription.canceled.v1",
		Payload:       payload,
	})
}
