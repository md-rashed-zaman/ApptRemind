package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/storage"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/webhook"
)

// StripeWebhook handles Stripe webhooks (no JWT auth; signature verification is the auth).
// Gateway should expose this path publicly.
func (h *Handler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.TrimSpace(h.stripeWebhookSecret) == "" {
		http.Error(w, "stripe webhook not configured", http.StatusServiceUnavailable)
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	if strings.TrimSpace(sigHeader) == "" {
		http.Error(w, "missing Stripe-Signature header", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB hard cap
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	evt, err := webhook.ConstructEventWithTolerance(body, sigHeader, h.stripeWebhookSecret, h.stripeWebhookTolerance)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	occurredAt := time.Unix(evt.Created, 0).UTC()
	evtType := string(evt.Type)
	h.logger.Info("billing provider event received",
		"provider", "stripe",
		"provider_event_id", evt.ID,
		"event_type", evtType,
		"occurred_at", occurredAt.UTC().Format(time.RFC3339),
	)

	tx, err := h.repo.Begin(r.Context())
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	// Idempotency: ignore replayed Stripe events.
	if err := h.repo.InsertProviderEvent(r.Context(), tx, storage.ProviderEvent{
		Provider:        "stripe",
		ProviderEventID: evt.ID,
		EventType:       evtType,
		Payload:         body,
	}); err != nil {
		if errors.Is(err, storage.ErrDuplicateProviderEvent) {
			h.logger.Info("billing provider event duplicate ignored", "provider", "stripe", "provider_event_id", evt.ID, "event_type", evtType)
			writeJSON(w, http.StatusOK, map[string]any{"status": "duplicate"})
			_ = tx.Commit(r.Context())
			return
		}
		http.Error(w, "failed to record provider event", http.StatusInternalServerError)
		return
	}

	if err := h.recordAudit(r.Context(), tx, r, "billing.provider.stripe.webhook", "provider", "", map[string]any{
		"provider":          "stripe",
		"provider_event_id": evt.ID,
		"event_type":        evtType,
		"occurred_at":       occurredAt.UTC().Format(time.RFC3339),
	}); err != nil {
		http.Error(w, "failed to record audit event", http.StatusInternalServerError)
		return
	}

	switch evtType {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		if err := json.Unmarshal(evt.Data.Raw, &session); err != nil {
			h.logger.Error("stripe: invalid checkout session payload", "err", err)
			break
		}
		businessID := strings.TrimSpace(session.Metadata["business_id"])
		tier := strings.TrimSpace(strings.ToLower(session.Metadata["tier"]))
		if businessID == "" || tier == "" {
			h.logger.Warn("stripe: missing metadata on checkout session (business_id/tier)")
			break
		}

		customerID := ""
		if session.Customer != nil {
			customerID = session.Customer.ID
		}
		subscriptionID := ""
		if session.Subscription != nil {
			subscriptionID = session.Subscription.ID
		}
		_ = h.repo.MarkCheckoutSessionCompleted(r.Context(), tx, session.ID, occurredAt, customerID, subscriptionID)
		if err := h.subSvc.ApplyActivated(r.Context(), tx, businessID, tier, occurredAt, "stripe", customerID, subscriptionID, nil, nil); err != nil {
			http.Error(w, "failed to apply activation", http.StatusInternalServerError)
			return
		}

	case "checkout.session.expired":
		var session stripe.CheckoutSession
		if err := json.Unmarshal(evt.Data.Raw, &session); err != nil {
			h.logger.Error("stripe: invalid checkout session payload", "err", err)
			break
		}
		_ = h.repo.MarkCheckoutSessionExpired(r.Context(), tx, session.ID, occurredAt)

	case "customer.subscription.created", "customer.subscription.updated":
		var sub stripe.Subscription
		if err := json.Unmarshal(evt.Data.Raw, &sub); err != nil {
			h.logger.Error("stripe: invalid subscription payload", "err", err)
			break
		}
		// Only treat active/trialing as entitled.
		if sub.Status != stripe.SubscriptionStatusActive && sub.Status != stripe.SubscriptionStatusTrialing {
			break
		}
		businessID := strings.TrimSpace(sub.Metadata["business_id"])
		tier := strings.TrimSpace(strings.ToLower(sub.Metadata["tier"]))
		if businessID == "" || tier == "" {
			h.logger.Warn("stripe: missing metadata on subscription (business_id/tier)")
			break
		}
		customerID := ""
		if sub.Customer != nil {
			customerID = sub.Customer.ID
		}
		var cps *time.Time
		var cpe *time.Time
		if sub.CurrentPeriodStart > 0 {
			t := time.Unix(sub.CurrentPeriodStart, 0).UTC()
			cps = &t
		}
		if sub.CurrentPeriodEnd > 0 {
			t := time.Unix(sub.CurrentPeriodEnd, 0).UTC()
			cpe = &t
		}
		if err := h.subSvc.ApplyActivated(r.Context(), tx, businessID, tier, occurredAt, "stripe", customerID, sub.ID, cps, cpe); err != nil {
			http.Error(w, "failed to apply activation", http.StatusInternalServerError)
			return
		}

	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(evt.Data.Raw, &sub); err != nil {
			h.logger.Error("stripe: invalid subscription payload", "err", err)
			break
		}
		businessID := strings.TrimSpace(sub.Metadata["business_id"])
		if businessID == "" {
			h.logger.Warn("stripe: missing metadata on subscription (business_id)")
			break
		}
		customerID := ""
		if sub.Customer != nil {
			customerID = sub.Customer.ID
		}
		var cps *time.Time
		var cpe *time.Time
		if sub.CurrentPeriodStart > 0 {
			t := time.Unix(sub.CurrentPeriodStart, 0).UTC()
			cps = &t
		}
		if sub.CurrentPeriodEnd > 0 {
			t := time.Unix(sub.CurrentPeriodEnd, 0).UTC()
			cpe = &t
		}
		if err := h.subSvc.ApplyCanceled(r.Context(), tx, businessID, occurredAt, "stripe", customerID, sub.ID, cps, cpe); err != nil {
			http.Error(w, "failed to apply cancellation", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, "failed to commit", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
