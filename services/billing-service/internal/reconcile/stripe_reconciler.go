package reconcile

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/storage"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/subscriptions"
	"github.com/stripe/stripe-go/v79"
	stripesubscription "github.com/stripe/stripe-go/v79/subscription"
)

type StripeReconciler struct {
	pool        *db.Pool
	repo        *storage.Repository
	subSvc      *subscriptions.Service
	logger      *slog.Logger
	stripeKey   string
	batchSize   int
	advisoryKey int64
}

type StripeReconcilerConfig struct {
	StripeSecretKey string
	Interval        time.Duration
	BatchSize       int
	AdvisoryLockKey int64
}

func NewStripeReconciler(pool *db.Pool, repo *storage.Repository, subSvc *subscriptions.Service, logger *slog.Logger, cfg StripeReconcilerConfig) *StripeReconciler {
	key := strings.TrimSpace(cfg.StripeSecretKey)
	bs := cfg.BatchSize
	if bs <= 0 {
		bs = 50
	}
	lockKey := cfg.AdvisoryLockKey
	if lockKey == 0 {
		// Stable-ish default; override via env if you run multiple billing instances.
		lockKey = 4242001
	}
	return &StripeReconciler{
		pool:        pool,
		repo:        repo,
		subSvc:      subSvc,
		logger:      logger,
		stripeKey:   key,
		batchSize:   bs,
		advisoryKey: lockKey,
	}
}

func (r *StripeReconciler) Run(ctx context.Context, interval time.Duration) {
	if strings.TrimSpace(r.stripeKey) == "" {
		r.logger.Warn("stripe reconcile disabled: STRIPE_SECRET_KEY missing")
		return
	}
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	// Best-effort leader election for multi-instance deployments.
	// Only the instance holding the advisory lock will reconcile.
	for {
		if ctx.Err() != nil {
			return
		}
		var locked bool
		if err := r.pool.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, r.advisoryKey).Scan(&locked); err != nil {
			r.logger.Error("stripe reconcile: failed to acquire advisory lock", "err", err)
			time.Sleep(5 * time.Second)
			continue
		}
		if !locked {
			r.logger.Info("stripe reconcile: advisory lock held by another instance", "lock_key", r.advisoryKey)
			time.Sleep(30 * time.Second)
			continue
		}
		r.logger.Info("stripe reconcile: advisory lock acquired", "lock_key", r.advisoryKey)
		defer func() {
			_, _ = r.pool.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, r.advisoryKey)
		}()
		break
	}

	stripe.Key = r.stripeKey
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on startup to self-heal faster after downtime.
	r.reconcileOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reconcileOnce(ctx)
		}
	}
}

func (r *StripeReconciler) reconcileOnce(ctx context.Context) {
	subs, err := r.repo.ListStripeSubscriptionsForReconcile(ctx, r.batchSize)
	if err != nil {
		r.logger.Error("stripe reconcile: failed to list subscriptions", "err", err)
		return
	}
	if len(subs) == 0 {
		return
	}

	for _, s := range subs {
		if ctx.Err() != nil {
			return
		}
		if strings.TrimSpace(s.StripeSubscriptionID) == "" || strings.TrimSpace(s.BusinessID) == "" {
			continue
		}

		stripeSub, err := stripesubscription.Get(s.StripeSubscriptionID, nil)
		if err != nil {
			r.logger.Warn("stripe reconcile: failed to fetch subscription", "err", err, "stripe_subscription_id", s.StripeSubscriptionID, "business_id", s.BusinessID)
			continue
		}

		customerID := ""
		if stripeSub.Customer != nil {
			customerID = stripeSub.Customer.ID
		}

		var cps *time.Time
		var cpe *time.Time
		if stripeSub.CurrentPeriodStart > 0 {
			t := time.Unix(stripeSub.CurrentPeriodStart, 0).UTC()
			cps = &t
		}
		if stripeSub.CurrentPeriodEnd > 0 {
			t := time.Unix(stripeSub.CurrentPeriodEnd, 0).UTC()
			cpe = &t
		}

		tier := strings.TrimSpace(strings.ToLower(stripeSub.Metadata["tier"]))
		if tier == "" {
			// If Stripe metadata is missing, keep the current tier rather than guessing.
			tier = s.Tier
		}

		// Stripe is the source of truth for lifecycle status.
		// We treat only active/trialing as entitled.
		entitled := stripeSub.Status == stripe.SubscriptionStatusActive || stripeSub.Status == stripe.SubscriptionStatusTrialing

		tx, err := r.repo.Begin(ctx)
		if err != nil {
			r.logger.Error("stripe reconcile: db begin failed", "err", err)
			return
		}

		applyErr := func() error {
			if entitled {
				occurredAt := time.Unix(stripeSub.Created, 0).UTC()
				return r.subSvc.ApplyActivated(ctx, tx, s.BusinessID, tier, occurredAt, "stripe", customerID, stripeSub.ID, cps, cpe)
			}
			occurredAt := time.Now().UTC()
			if stripeSub.CanceledAt > 0 {
				occurredAt = time.Unix(stripeSub.CanceledAt, 0).UTC()
			}
			return r.subSvc.ApplyCanceled(ctx, tx, s.BusinessID, occurredAt, "stripe", customerID, stripeSub.ID, cps, cpe)
		}()

		if applyErr != nil {
			_ = tx.Rollback(ctx)
			r.logger.Warn("stripe reconcile: apply failed", "err", applyErr, "business_id", s.BusinessID, "stripe_subscription_id", stripeSub.ID)
			continue
		}
		if err := tx.Commit(ctx); err != nil {
			_ = tx.Rollback(ctx)
			r.logger.Warn("stripe reconcile: commit failed", "err", err, "business_id", s.BusinessID, "stripe_subscription_id", stripeSub.ID)
			continue
		}
	}
}
