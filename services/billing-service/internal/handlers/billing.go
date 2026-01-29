package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/entitlements"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/outbox"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/storage"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/subscriptions"
	"github.com/stripe/stripe-go/v79"
	checkoutsession "github.com/stripe/stripe-go/v79/checkout/session"
	stripesubscription "github.com/stripe/stripe-go/v79/subscription"
)

type Handler struct {
	repo                   *storage.Repository
	outboxRepo             *outbox.Repository
	subSvc                 *subscriptions.Service
	logger                 *slog.Logger
	stripeWebhookSecret    string
	stripeWebhookTolerance time.Duration
	stripeSecretKey        string
	stripePriceStarter     string
	stripePricePro         string
	checkoutSuccessURL     string
	checkoutCancelURL      string
}

type Config struct {
	StripeWebhookSecret           string
	StripeWebhookToleranceSeconds int
	StripeSecretKey               string
	StripePriceStarter            string
	StripePricePro                string
	CheckoutSuccessURL            string
	CheckoutCancelURL             string
}

func New(repo *storage.Repository, outboxRepo *outbox.Repository, logger *slog.Logger, cfg Config) *Handler {
	tolSeconds := cfg.StripeWebhookToleranceSeconds
	if tolSeconds <= 0 {
		tolSeconds = 300
	}
	return &Handler{
		repo:                   repo,
		outboxRepo:             outboxRepo,
		subSvc:                 subscriptions.New(repo, outboxRepo),
		logger:                 logger,
		stripeWebhookSecret:    strings.TrimSpace(cfg.StripeWebhookSecret),
		stripeWebhookTolerance: time.Duration(tolSeconds) * time.Second,
		stripeSecretKey:        strings.TrimSpace(cfg.StripeSecretKey),
		stripePriceStarter:     strings.TrimSpace(cfg.StripePriceStarter),
		stripePricePro:         strings.TrimSpace(cfg.StripePricePro),
		checkoutSuccessURL:     strings.TrimSpace(cfg.CheckoutSuccessURL),
		checkoutCancelURL:      strings.TrimSpace(cfg.CheckoutCancelURL),
	}
}

type localWebhookRequest struct {
	EventID    string `json:"event_id"`
	Type       string `json:"type"` // subscription.activated | subscription.canceled
	BusinessID string `json:"business_id"`
	Tier       string `json:"tier"`
	OccurredAt string `json:"occurred_at"`
}

func (h *Handler) LocalWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req localWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	req.EventID = strings.TrimSpace(req.EventID)
	req.Type = strings.TrimSpace(req.Type)
	req.BusinessID = strings.TrimSpace(req.BusinessID)
	req.Tier = strings.TrimSpace(strings.ToLower(req.Tier))
	req.OccurredAt = strings.TrimSpace(req.OccurredAt)

	if req.EventID == "" || req.Type == "" || req.BusinessID == "" || req.OccurredAt == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	occurredAt, err := time.Parse(time.RFC3339, req.OccurredAt)
	if err != nil {
		http.Error(w, "invalid occurred_at", http.StatusBadRequest)
		return
	}

	h.logger.Info("billing provider event received",
		"provider", "local",
		"provider_event_id", req.EventID,
		"event_type", req.Type,
		"business_id", req.BusinessID,
		"tier", req.Tier,
		"occurred_at", occurredAt.UTC().Format(time.RFC3339),
	)

	role := r.Header.Get("X-Role")
	callerBusinessID := r.Header.Get("X-Business-Id")
	if role != "admin" && callerBusinessID != "" && callerBusinessID != req.BusinessID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	payloadRaw, _ := json.Marshal(req)

	tx, err := h.repo.Begin(r.Context())
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	if err := h.repo.InsertProviderEvent(r.Context(), tx, storage.ProviderEvent{
		Provider:        "local",
		ProviderEventID: req.EventID,
		EventType:       req.Type,
		Payload:         payloadRaw,
	}); err != nil {
		if errors.Is(err, storage.ErrDuplicateProviderEvent) {
			h.logger.Info("billing provider event duplicate ignored", "provider", "local", "provider_event_id", req.EventID, "event_type", req.Type)
			writeJSON(w, http.StatusOK, map[string]any{"status": "duplicate"})
			_ = tx.Commit(r.Context())
			return
		}
		http.Error(w, "failed to record provider event", http.StatusInternalServerError)
		return
	}

	if err := h.recordAudit(r.Context(), tx, r, "billing.provider.local.webhook", "provider", req.BusinessID, map[string]any{
		"provider":          "local",
		"provider_event_id": req.EventID,
		"event_type":        req.Type,
		"tier":              req.Tier,
		"occurred_at":       occurredAt.UTC().Format(time.RFC3339),
	}); err != nil {
		http.Error(w, "failed to record audit event", http.StatusInternalServerError)
		return
	}

	switch req.Type {
	case "subscription.activated":
		if req.Tier == "" {
			http.Error(w, "tier is required for subscription.activated", http.StatusBadRequest)
			return
		}
		if err := h.subSvc.ApplyActivated(r.Context(), tx, req.BusinessID, req.Tier, occurredAt, "local", "", "", nil, nil); err != nil {
			http.Error(w, "failed to apply activation", http.StatusInternalServerError)
			return
		}
	case "subscription.canceled":
		if err := h.subSvc.ApplyCanceled(r.Context(), tx, req.BusinessID, occurredAt, "local", "", "", nil, nil); err != nil {
			http.Error(w, "failed to apply cancellation", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "unsupported type", http.StatusBadRequest)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, "failed to commit", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := strings.TrimSpace(r.URL.Query().Get("business_id"))
	if businessID == "" {
		businessID = strings.TrimSpace(r.Header.Get("X-Business-Id"))
	}
	if businessID == "" {
		http.Error(w, "business_id is required", http.StatusBadRequest)
		return
	}

	role := r.Header.Get("X-Role")
	callerBusinessID := r.Header.Get("X-Business-Id")
	if role != "admin" && callerBusinessID != "" && callerBusinessID != businessID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	sub, err := h.repo.GetSubscription(r.Context(), businessID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Not found -> return free defaults (better DX).
			writeJSON(w, http.StatusOK, map[string]any{
				"business_id":  businessID,
				"tier":         "free",
				"status":       "none",
				"entitlements": entitlements.LimitsForTier("free"),
			})
			return
		}
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"business_id":  businessID,
		"tier":         sub.Tier,
		"status":       sub.Status,
		"updated_at":   sub.UpdatedAt.UTC().Format(time.RFC3339),
		"entitlements": entitlements.LimitsForTier(sub.Tier),
	})
}

type cancelSubscriptionRequest struct {
	BusinessID string `json:"business_id,omitempty"` // admin only
}

func (h *Handler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.stripeSecretKey == "" {
		http.Error(w, "stripe billing not configured (STRIPE_SECRET_KEY missing)", http.StatusNotImplemented)
		return
	}

	var req cancelSubscriptionRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // optional body
	req.BusinessID = strings.TrimSpace(req.BusinessID)

	role := r.Header.Get("X-Role")
	callerBusinessID := strings.TrimSpace(r.Header.Get("X-Business-Id"))

	businessID := callerBusinessID
	if role == "admin" && req.BusinessID != "" {
		businessID = req.BusinessID
	}
	if businessID == "" {
		http.Error(w, "business_id is required", http.StatusBadRequest)
		return
	}
	if role != "admin" && callerBusinessID != "" && callerBusinessID != businessID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	sub, err := h.repo.GetSubscription(r.Context(), businessID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "subscription not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	stripeSubID := strings.TrimSpace(sub.StripeSubscriptionID)
	if stripeSubID == "" {
		http.Error(w, "no stripe subscription id on record", http.StatusConflict)
		return
	}

	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idemKey == "" {
		// Deterministic fallback prevents accidental duplicates when clients don't send Idempotency-Key.
		idemKey = "cancel:" + businessID + ":" + stripeSubID
	}

	stripe.Key = h.stripeSecretKey
	cancelParams := &stripe.SubscriptionCancelParams{}
	cancelParams.IdempotencyKey = stripe.String(idemKey)

	cancelCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	// stripe-go uses context via stripe.Backend? It doesn't accept ctx directly; timeout is still useful for our handler.
	_ = cancelCtx

	stripeSub, err := stripesubscription.Cancel(stripeSubID, cancelParams)
	if err != nil {
		h.logger.Error("stripe subscription cancel failed", "err", err, "stripe_subscription_id", stripeSubID)
		http.Error(w, "failed to cancel subscription", http.StatusBadGateway)
		return
	}

	now := time.Now().UTC()
	customerID := ""
	if stripeSub != nil && stripeSub.Customer != nil {
		customerID = stripeSub.Customer.ID
	}

	tx, err := h.repo.Begin(r.Context())
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	payload, _ := json.Marshal(map[string]any{
		"business_id":            businessID,
		"stripe_subscription_id": stripeSubID,
		"idempotency_key":        idemKey,
		"canceled_at":            now.Format(time.RFC3339),
	})
	if err := h.repo.InsertProviderEvent(r.Context(), tx, storage.ProviderEvent{
		Provider:        "internal",
		ProviderEventID: idemKey,
		EventType:       "subscription.cancel",
		Payload:         payload,
	}); err != nil {
		if errors.Is(err, storage.ErrDuplicateProviderEvent) {
			writeJSON(w, http.StatusOK, map[string]any{"status": "duplicate"})
			_ = tx.Commit(r.Context())
			return
		}
		http.Error(w, "failed to record cancellation", http.StatusInternalServerError)
		return
	}

	if err := h.recordAudit(r.Context(), tx, r, "billing.subscription.cancel.requested", "", businessID, map[string]any{
		"provider":               "stripe",
		"stripe_subscription_id": stripeSubID,
		"idempotency_key":        idemKey,
	}); err != nil {
		http.Error(w, "failed to record audit event", http.StatusInternalServerError)
		return
	}

	if err := h.subSvc.ApplyCanceled(r.Context(), tx, businessID, now, "stripe", customerID, stripeSubID, nil, nil); err != nil {
		http.Error(w, "failed to apply cancellation", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, "failed to commit", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

type checkoutRequest struct {
	Tier       string `json:"tier"`
	SuccessURL string `json:"success_url,omitempty"`
	CancelURL  string `json:"cancel_url,omitempty"`
}

func (h *Handler) CheckoutStub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.stripeSecretKey == "" {
		http.Error(w, "stripe checkout not configured (STRIPE_SECRET_KEY missing)", http.StatusNotImplemented)
		return
	}

	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	tier := strings.TrimSpace(strings.ToLower(req.Tier))
	if tier == "" {
		http.Error(w, "tier is required", http.StatusBadRequest)
		return
	}

	businessID := strings.TrimSpace(r.Header.Get("X-Business-Id"))
	if businessID == "" {
		http.Error(w, "missing business context", http.StatusBadRequest)
		return
	}

	priceID := ""
	switch tier {
	case "starter":
		priceID = h.stripePriceStarter
	case "pro":
		priceID = h.stripePricePro
	default:
		http.Error(w, "unsupported tier", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(priceID) == "" {
		http.Error(w, "stripe price id not configured for tier", http.StatusNotImplemented)
		return
	}

	successURL := strings.TrimSpace(req.SuccessURL)
	if successURL == "" {
		successURL = h.checkoutSuccessURL
	}
	cancelURL := strings.TrimSpace(req.CancelURL)
	if cancelURL == "" {
		cancelURL = h.checkoutCancelURL
	}
	if successURL == "" || cancelURL == "" {
		http.Error(w, "success_url and cancel_url are required (or configure default URLs)", http.StatusBadRequest)
		return
	}

	// Protect the public return pages from session-id guessing / tampering.
	returnToken := newReturnToken()
	successURL = withQueryParam(successURL, "state", returnToken)
	cancelURL = withQueryParam(cancelURL, "state", returnToken)

	// Stripe uses a global API key. Keep usage limited to this handler call.
	stripe.Key = h.stripeSecretKey

	// Stripe-level idempotency: allows safe retries.
	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))

	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		ClientReferenceID: stripe.String(businessID),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"business_id": businessID,
			"tier":        tier,
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"business_id": businessID,
				"tier":        tier,
			},
		},
	}
	params.AddExpand("url")
	if idemKey != "" {
		params.IdempotencyKey = stripe.String(idemKey)
	}

	sess, err := checkoutsession.New(params)
	if err != nil {
		h.logger.Error("stripe checkout session create failed", "err", err)
		http.Error(w, "failed to create checkout session", http.StatusBadGateway)
		return
	}

	tx, err := h.repo.Begin(r.Context())
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if err := h.repo.UpsertCheckoutSession(r.Context(), tx, storage.CheckoutSession{
		StripeSessionID: sess.ID,
		BusinessID:      businessID,
		Tier:            tier,
		Status:          "created",
		URL:             sess.URL,
		ReturnToken:     returnToken,
	}); err != nil {
		http.Error(w, "failed to persist checkout session", http.StatusInternalServerError)
		return
	}
	if err := h.recordAudit(r.Context(), tx, r, "billing.checkout.created", "", businessID, map[string]any{
		"tier":              tier,
		"stripe_session_id": sess.ID,
	}); err != nil {
		http.Error(w, "failed to record audit event", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, "failed to commit", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sess.ID,
		"url":        sess.URL,
	})
}

// CheckoutSessionStatus is intentionally public: Stripe redirects the customer without a JWT.
// It returns non-sensitive state only.
func (h *Handler) CheckoutSessionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}

	sess, err := h.repo.GetCheckoutSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load session", http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"session_id": sess.StripeSessionID,
		"tier":       sess.Tier,
		"status":     sess.Status,
		"updated_at": sess.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if sess.CompletedAt != nil {
		resp["completed_at"] = sess.CompletedAt.UTC().Format(time.RFC3339)
	}
	if sess.CanceledAt != nil {
		resp["canceled_at"] = sess.CanceledAt.UTC().Format(time.RFC3339)
	}
	if sess.ExpiredAt != nil {
		resp["expired_at"] = sess.ExpiredAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}

type checkoutAckRequest struct {
	SessionID string `json:"session_id"`
	State     string `json:"state"`
	Result    string `json:"result"` // success | cancel
}

// AckCheckoutReturn is public but protected by the per-session return_token (state).
func (h *Handler) AckCheckoutReturn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req checkoutAckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.State = strings.TrimSpace(req.State)
	req.Result = strings.TrimSpace(strings.ToLower(req.Result))
	if req.SessionID == "" || req.State == "" {
		http.Error(w, "session_id and state are required", http.StatusBadRequest)
		return
	}
	if req.Result != "success" && req.Result != "cancel" {
		http.Error(w, "invalid result", http.StatusBadRequest)
		return
	}

	tx, err := h.repo.Begin(r.Context())
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	if err := h.repo.AckCheckoutReturn(r.Context(), tx, req.SessionID, req.State, req.Result, time.Now().UTC()); err != nil {
		http.Error(w, "failed to record return", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, "failed to commit", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func newReturnToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	// URL-safe, no padding.
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func withQueryParam(rawURL string, key string, value string) string {
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return rawURL + sep + key + "=" + url.QueryEscape(value)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) recordAudit(ctx context.Context, tx pgx.Tx, r *http.Request, eventType string, actorType string, businessID string, metadata map[string]any) error {
	if actorType == "" {
		actorType = strings.TrimSpace(r.Header.Get("X-Role"))
	}
	if actorType == "" {
		actorType = "system"
	}
	actorID := strings.TrimSpace(r.Header.Get("X-User-Id"))
	if actorID == "" {
		actorID = strings.TrimSpace(r.Header.Get("X-Business-Id"))
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	if reqID := strings.TrimSpace(r.Header.Get("X-Request-Id")); reqID != "" {
		metadata["request_id"] = reqID
	}
	raw, _ := json.Marshal(metadata)
	return h.repo.InsertAuditEvent(ctx, tx, storage.AuditEvent{
		EventType:  eventType,
		ActorType:  actorType,
		ActorID:    actorID,
		BusinessID: businessID,
		Metadata:   raw,
	})
}
