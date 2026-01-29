package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/availability"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/model"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/outbox"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/policy"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/scheduling"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/storage"
)

type BookingHandler struct {
	repo       *storage.BookingRepository
	outboxRepo *outbox.Repository
	logger     *slog.Logger
	policy     policy.Provider
	scheduling scheduling.Provider
	defaults   []time.Duration
}

func NewBookingHandler(repo *storage.BookingRepository, outboxRepo *outbox.Repository, logger *slog.Logger, policyProvider policy.Provider, schedulingProvider scheduling.Provider, defaults []time.Duration) *BookingHandler {
	return &BookingHandler{
		repo:       repo,
		outboxRepo: outboxRepo,
		logger:     logger,
		policy:     policyProvider,
		scheduling: schedulingProvider,
		defaults:   defaults,
	}
}

type createBookingRequest struct {
	BusinessID    string `json:"business_id"`
	ServiceID     string `json:"service_id"`
	StaffID       string `json:"staff_id"`
	CustomerName  string `json:"customer_name"`
	CustomerEmail string `json:"customer_email"`
	CustomerPhone string `json:"customer_phone"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
}

type createBookingResponse struct {
	AppointmentID string `json:"appointment_id"`
}

type cancelBookingRequest struct {
	BusinessID    string `json:"business_id"`
	AppointmentID string `json:"appointment_id"`
	Reason        string `json:"reason"`
}

type cancelBookingResponse struct {
	AppointmentID string `json:"appointment_id"`
	Status        string `json:"status"`
	CancelledAt   string `json:"cancelled_at"`
}

type listAppointmentItem struct {
	AppointmentID string `json:"appointment_id"`
	StaffID       string `json:"staff_id"`
	ServiceID     string `json:"service_id"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
	Status        string `json:"status"`
	CancelledAt   string `json:"cancelled_at,omitempty"`
	CreatedAt     string `json:"created_at"`
}

type slotItem struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

func (h *BookingHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req createBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	req.BusinessID = strings.TrimSpace(req.BusinessID)
	req.ServiceID = strings.TrimSpace(req.ServiceID)
	req.StaffID = strings.TrimSpace(req.StaffID)
	req.CustomerName = strings.TrimSpace(req.CustomerName)

	if req.BusinessID == "" || req.ServiceID == "" || req.StaffID == "" || req.CustomerName == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		http.Error(w, "invalid start_time", http.StatusBadRequest)
		return
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		http.Error(w, "invalid end_time", http.StatusBadRequest)
		return
	}
	if !endTime.After(startTime) {
		http.Error(w, "end_time must be after start_time", http.StatusBadRequest)
		return
	}

	appt := &model.Appointment{
		BusinessID:    req.BusinessID,
		ServiceID:     req.ServiceID,
		StaffID:       req.StaffID,
		CustomerName:  req.CustomerName,
		CustomerEmail: strings.TrimSpace(req.CustomerEmail),
		CustomerPhone: strings.TrimSpace(req.CustomerPhone),
		StartTime:     startTime,
		EndTime:       endTime,
		Status:        "booked",
	}

	ctx := r.Context()
	tx, err := h.repo.Begin(ctx)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey != "" {
		rec, exists, err := h.repo.LockIdempotencyKey(ctx, tx, appt.BusinessID, idempotencyKey)
		if err != nil {
			http.Error(w, "failed to lock idempotency key", http.StatusInternalServerError)
			return
		}
		if exists && rec.AppointmentID != "" && rec.StatusCode > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(rec.StatusCode)
			if len(rec.ResponsePayload) > 0 {
				_, _ = w.Write(rec.ResponsePayload)
				return
			}
			_ = json.NewEncoder(w).Encode(createBookingResponse{AppointmentID: rec.AppointmentID})
			return
		}
	}

	// Production guardrail: bookings must fit within staff availability windows
	// (working hours minus time-off blackouts) when the scheduling provider is enabled.
	ok, err := h.validateBookingWithinAvailability(ctx, appt)
	if err != nil {
		// Do not finalize idempotency on dependency errors; allow the client to retry later with the same key.
		http.Error(w, "availability service unavailable", http.StatusServiceUnavailable)
		return
	}
	if !ok {
		if idempotencyKey != "" {
			if h.finalizeIdempotencyError(ctx, tx, appt.BusinessID, idempotencyKey, http.StatusUnprocessableEntity, "requested time is outside business availability") {
				_ = tx.Commit(ctx)
				return
			}
		}
		http.Error(w, "requested time is outside business availability", http.StatusUnprocessableEntity)
		return
	}

	// Enforce billing entitlements (MVP): cap monthly booked appointments per business.
	// If entitlements aren't present yet, default to free tier limits.
	if err := h.enforceMonthlyAppointmentLimit(ctx, tx, appt.BusinessID, appt.StartTime); err != nil {
		if errors.Is(err, errPaymentRequired) {
			if idempotencyKey != "" {
				if h.finalizeIdempotencyError(ctx, tx, appt.BusinessID, idempotencyKey, http.StatusPaymentRequired, err.Error()) {
					_ = tx.Commit(ctx)
					return
				}
			}
			http.Error(w, err.Error(), http.StatusPaymentRequired)
			return
		}
		http.Error(w, "entitlements check failed", http.StatusInternalServerError)
		return
	}

	id, err := h.repo.Create(ctx, tx, appt)
	if err != nil {
		if storage.IsConflict(err) {
			http.Error(w, "time slot already booked", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create appointment", http.StatusInternalServerError)
		return
	}

	evtPayload, err := json.Marshal(map[string]any{
		"appointment_id": id,
		"business_id":    appt.BusinessID,
		"staff_id":       appt.StaffID,
		"service_id":     appt.ServiceID,
		"customer_email": appt.CustomerEmail,
		"customer_phone": appt.CustomerPhone,
		"start_time":     appt.StartTime.Format(time.RFC3339),
		"end_time":       appt.EndTime.Format(time.RFC3339),
	})
	if err != nil {
		http.Error(w, "failed to build event payload", http.StatusInternalServerError)
		return
	}

	if err := h.outboxRepo.Insert(ctx, tx, outbox.Event{
		AggregateType: "appointment",
		AggregateID:   id,
		EventType:     "booking.appointment.booked.v1",
		Payload:       evtPayload,
	}); err != nil {
		http.Error(w, "failed to write outbox event", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	offsets := h.defaults
	if h.policy != nil {
		if policyOffsets, err := h.policy.ReminderOffsets(r.Context(), appt.BusinessID); err == nil && len(policyOffsets) > 0 {
			offsets = policyOffsets
		} else if err != nil {
			h.logger.Warn("policy offsets fetch failed; using defaults", "err", err)
		}
	}
	for _, offset := range offsets {
		remindAt := appt.StartTime.Add(-offset)
		if remindAt.Before(now) {
			continue
		}
		h.enqueueReminder(ctx, tx, id, appt, remindAt, "email", appt.CustomerEmail)
		h.enqueueReminder(ctx, tx, id, appt, remindAt, "sms", appt.CustomerPhone)
	}

	respBody, err := json.Marshal(createBookingResponse{AppointmentID: id})
	if err != nil {
		http.Error(w, "failed to build response", http.StatusInternalServerError)
		return
	}
	if idempotencyKey != "" {
		if err := h.repo.FinalizeIdempotency(ctx, tx, appt.BusinessID, idempotencyKey, id, http.StatusCreated, respBody); err != nil {
			http.Error(w, "failed to finalize idempotency key", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "failed to commit", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(respBody)
}

var errPaymentRequired = errors.New("monthly appointment limit reached (upgrade required)")

func (h *BookingHandler) enforceMonthlyAppointmentLimit(ctx context.Context, tx pgx.Tx, businessID string, start time.Time) error {
	const defaultFreeMax = 200

	ent, ok, err := h.repo.GetBusinessEntitlements(ctx, tx, businessID)
	if err != nil {
		return err
	}
	max := defaultFreeMax
	if ok && ent.MaxMonthlyAppointments > 0 {
		max = ent.MaxMonthlyAppointments
	}
	if max <= 0 {
		return nil
	}

	startUTC := start.UTC()
	monthStart := time.Date(startUTC.Year(), startUTC.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	cnt, err := h.repo.CountBookedByBusinessInRange(ctx, tx, businessID, monthStart, monthEnd)
	if err != nil {
		return err
	}
	if cnt >= max {
		return errPaymentRequired
	}
	return nil
}

func (h *BookingHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req cancelBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.BusinessID = strings.TrimSpace(req.BusinessID)
	req.AppointmentID = strings.TrimSpace(req.AppointmentID)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.BusinessID == "" || req.AppointmentID == "" {
		http.Error(w, "business_id and appointment_id required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	tx, err := h.repo.Begin(ctx)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	appt, err := h.repo.GetAppointmentForUpdate(ctx, tx, req.BusinessID, req.AppointmentID)
	if err != nil {
		if storage.IsNotFound(err) {
			http.Error(w, "appointment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load appointment", http.StatusInternalServerError)
		return
	}

	if appt.Status == "cancelled" && appt.CancelledAt != nil {
		h.writeCancelResponse(w, appt.ID, appt.CancelledAt.UTC())
		return
	}
	if appt.Status != "booked" {
		http.Error(w, "appointment cannot be cancelled", http.StatusConflict)
		return
	}

	cancelledAt, err := h.repo.CancelAppointment(ctx, tx, req.BusinessID, appt.ID, req.Reason)
	if err != nil {
		http.Error(w, "failed to cancel appointment", http.StatusInternalServerError)
		return
	}

	cancelPayload, err := json.Marshal(map[string]any{
		"appointment_id": appt.ID,
		"business_id":    appt.BusinessID,
		"staff_id":       appt.StaffID,
		"service_id":     appt.ServiceID,
		"start_time":     appt.StartTime.UTC().Format(time.RFC3339),
		"end_time":       appt.EndTime.UTC().Format(time.RFC3339),
		"cancelled_at":   cancelledAt.UTC().Format(time.RFC3339),
		"reason":         req.Reason,
	})
	if err != nil {
		http.Error(w, "failed to build cancellation event", http.StatusInternalServerError)
		return
	}
	if err := h.outboxRepo.Insert(ctx, tx, outbox.Event{
		AggregateType: "appointment",
		AggregateID:   appt.ID,
		EventType:     "booking.appointment.cancelled.v1",
		Payload:       cancelPayload,
	}); err != nil {
		http.Error(w, "failed to write outbox event", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "failed to commit", http.StatusInternalServerError)
		return
	}
	h.writeCancelResponse(w, appt.ID, cancelledAt.UTC())
}

func (h *BookingHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := strings.TrimSpace(r.Header.Get("X-Business-Id"))
	if businessID == "" {
		businessID = strings.TrimSpace(r.URL.Query().Get("business_id"))
	}
	if businessID == "" {
		http.Error(w, "business_id required", http.StatusBadRequest)
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	appts, err := h.repo.ListByBusiness(r.Context(), businessID, limit)
	if err != nil {
		http.Error(w, "failed to list appointments", http.StatusInternalServerError)
		return
	}

	items := make([]listAppointmentItem, 0, len(appts))
	for _, appt := range appts {
		item := listAppointmentItem{
			AppointmentID: appt.ID,
			StaffID:       appt.StaffID,
			ServiceID:     appt.ServiceID,
			StartTime:     appt.StartTime.UTC().Format(time.RFC3339),
			EndTime:       appt.EndTime.UTC().Format(time.RFC3339),
			Status:        appt.Status,
			CreatedAt:     appt.CreatedAt.UTC().Format(time.RFC3339),
		}
		if appt.CancelledAt != nil {
			item.CancelledAt = appt.CancelledAt.UTC().Format(time.RFC3339)
		}
		items = append(items, item)
	}

	body, err := json.Marshal(items)
	if err != nil {
		http.Error(w, "failed to build response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *BookingHandler) Slots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := strings.TrimSpace(r.URL.Query().Get("business_id"))
	staffID := strings.TrimSpace(r.URL.Query().Get("staff_id"))
	serviceID := strings.TrimSpace(r.URL.Query().Get("service_id"))
	dateStr := strings.TrimSpace(r.URL.Query().Get("date"))
	if businessID == "" || staffID == "" || dateStr == "" || serviceID == "" {
		http.Error(w, "business_id, staff_id, service_id, and date are required", http.StatusBadRequest)
		return
	}

	windows, durationMins, stepMins, ok := h.resolveAvailabilityWindows(r.Context(), businessID, staffID, serviceID, dateStr, r)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
		return
	}

	minStart, maxEnd := minMaxWindows(windows)
	if minStart.IsZero() || maxEnd.IsZero() || !maxEnd.After(minStart) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
		return
	}

	// Load booked intervals for the staff/day window. Cancelled appointments do not block.
	booked, err := h.repo.ListBookedIntervals(r.Context(), businessID, staffID, minStart, maxEnd)
	if err != nil {
		http.Error(w, "failed to load booked slots", http.StatusInternalServerError)
		return
	}

	busy := make([]availability.Interval, 0, len(booked))
	for _, a := range booked {
		busy = append(busy, availability.Interval{Start: a.StartTime, End: a.EndTime})
	}

	var resp []slotItem
	for _, win := range windows {
		slotStarts := availability.AvailableSlots(
			win.Start,
			win.End,
			time.Duration(durationMins)*time.Minute,
			time.Duration(stepMins)*time.Minute,
			busy,
			time.Now().UTC(),
		)
		for _, s := range slotStarts {
			resp = append(resp, slotItem{
				StartTime: s.UTC().Format(time.RFC3339),
				EndTime:   s.Add(time.Duration(durationMins) * time.Minute).UTC().Format(time.RFC3339),
			})
		}
	}

	body, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "failed to build response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *BookingHandler) resolveAvailabilityWindows(ctx context.Context, businessID, staffID, serviceID, dateStr string, r *http.Request) ([]availability.Interval, int, int, bool) {
	// Try business-service gRPC when available (production path).
	if h.scheduling != nil {
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		cfg, err := h.scheduling.GetAvailabilityConfig(reqCtx, businessID, staffID, serviceID, dateStr)
		if err == nil {
			if !cfg.IsWorking {
				return nil, 0, 0, false
			}
			duration := cfg.DurationMinutes
			if duration <= 0 {
				duration = 30
			}
			step := cfg.SlotStepMinutes
			if step <= 0 {
				step = 15
			}

			// Prefer explicit windows when provided (work hours with time off subtracted).
			if len(cfg.WindowsUTC) > 0 {
				wins := make([]availability.Interval, 0, len(cfg.WindowsUTC))
				for _, w := range cfg.WindowsUTC {
					start := w.StartUTC.UTC()
					end := w.EndUTC.UTC()
					if end.After(start) {
						wins = append(wins, availability.Interval{Start: start, End: end})
					}
				}
				if len(wins) > 0 {
					return wins, duration, step, true
				}
				return nil, duration, step, false
			}

			// Back-compat: single window.
			if cfg.WorkStartUTC.IsZero() || cfg.WorkEndUTC.IsZero() || !cfg.WorkEndUTC.After(cfg.WorkStartUTC) {
				return nil, 0, 0, false
			}
			return []availability.Interval{{Start: cfg.WorkStartUTC.UTC(), End: cfg.WorkEndUTC.UTC()}}, duration, step, true
		}
		h.logger.Warn("availability config fetch failed; falling back to query params", "err", err)
	}

	// Fallback: allow explicit duration/step and workday hours for dev/testing without business-service.
	durationMins := 30
	if v := strings.TrimSpace(r.URL.Query().Get("duration_minutes")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 8*60 {
			durationMins = n
		} else {
			return nil, 0, 0, false
		}
	}
	stepMins := 15
	if v := strings.TrimSpace(r.URL.Query().Get("slot_step_minutes")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 120 {
			stepMins = n
		} else {
			return nil, 0, 0, false
		}
	}
	workStart := strings.TrimSpace(r.URL.Query().Get("workday_start"))
	if workStart == "" {
		workStart = "09:00"
	}
	workEnd := strings.TrimSpace(r.URL.Query().Get("workday_end"))
	if workEnd == "" {
		workEnd = "17:00"
	}

	day, err := time.ParseInLocation("2006-01-02", dateStr, time.UTC)
	if err != nil {
		return nil, 0, 0, false
	}
	startClock, err := time.Parse("15:04", workStart)
	if err != nil {
		return nil, 0, 0, false
	}
	endClock, err := time.Parse("15:04", workEnd)
	if err != nil {
		return nil, 0, 0, false
	}
	windowStart := time.Date(day.Year(), day.Month(), day.Day(), startClock.Hour(), startClock.Minute(), 0, 0, time.UTC)
	windowEnd := time.Date(day.Year(), day.Month(), day.Day(), endClock.Hour(), endClock.Minute(), 0, 0, time.UTC)
	if !windowEnd.After(windowStart) {
		return nil, 0, 0, false
	}
	return []availability.Interval{{Start: windowStart, End: windowEnd}}, durationMins, stepMins, true
}

func minMaxWindows(windows []availability.Interval) (time.Time, time.Time) {
	var min time.Time
	var max time.Time
	for _, w := range windows {
		if w.Start.IsZero() || w.End.IsZero() || !w.End.After(w.Start) {
			continue
		}
		if min.IsZero() || w.Start.Before(min) {
			min = w.Start
		}
		if max.IsZero() || w.End.After(max) {
			max = w.End
		}
	}
	return min, max
}

func (h *BookingHandler) enqueueReminder(ctx context.Context, tx pgx.Tx, appointmentID string, appt *model.Appointment, remindAt time.Time, channel string, recipient string) {
	if strings.TrimSpace(recipient) == "" {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"appointment_id": appointmentID,
		"business_id":    appt.BusinessID,
		"channel":        channel,
		"recipient":      recipient,
		"remind_at":      remindAt.UTC().Format(time.RFC3339),
		"template_data": map[string]any{
			"customer_name": appt.CustomerName,
			"service_id":    appt.ServiceID,
			"start_time":    appt.StartTime.UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		h.logger.Error("failed to build reminder payload", "err", err)
		return
	}
	if err := h.outboxRepo.Insert(ctx, tx, outbox.Event{
		AggregateType: "appointment",
		AggregateID:   appointmentID,
		EventType:     "booking.reminder.requested.v1",
		Payload:       payload,
	}); err != nil {
		h.logger.Error("failed to enqueue reminder", "err", err)
	}
}

func (h *BookingHandler) writeCancelResponse(w http.ResponseWriter, appointmentID string, cancelledAt time.Time) {
	resp := cancelBookingResponse{
		AppointmentID: appointmentID,
		Status:        "cancelled",
		CancelledAt:   cancelledAt.Format(time.RFC3339),
	}
	body, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "failed to build response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *BookingHandler) finalizeIdempotencyError(ctx context.Context, tx pgx.Tx, businessID, key string, statusCode int, msg string) bool {
	body, err := json.Marshal(map[string]string{"error": msg})
	if err != nil {
		return false
	}
	if err := h.repo.FinalizeIdempotency(ctx, tx, businessID, key, "", statusCode, body); err != nil {
		h.logger.Error("failed to finalize idempotency (error)", "err", err)
		return false
	}
	return true
}

func (h *BookingHandler) validateBookingWithinAvailability(ctx context.Context, appt *model.Appointment) (bool, error) {
	if h.scheduling == nil {
		// No scheduling provider in this build; rely on DB overlap constraint only.
		return true, nil
	}

	startUTC := appt.StartTime.UTC()
	endUTC := appt.EndTime.UTC()
	if !endUTC.After(startUTC) {
		return false, nil
	}

	// The business-service API expects a business-local date (YYYY-MM-DD). Without knowing the business TZ
	// up front, query a small set of candidates (UTC date +/- 1 day) and accept if any availability window
	// contains the requested booking interval.
	dates := uniqueStrings([]string{
		startUTC.Add(-24 * time.Hour).Format("2006-01-02"),
		startUTC.Format("2006-01-02"),
		startUTC.Add(24 * time.Hour).Format("2006-01-02"),
	})

	var lastErr error
	for _, dateStr := range dates {
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		cfg, err := h.scheduling.GetAvailabilityConfig(reqCtx, appt.BusinessID, appt.StaffID, appt.ServiceID, dateStr)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		if !cfg.IsWorking {
			// Could be closed or fully blocked by time off; keep checking other date candidates.
			continue
		}

		wins := windowsFromConfig(cfg)
		for _, w := range wins {
			if !w.End.After(w.Start) {
				continue
			}
			if !startUTC.Before(w.Start) && !endUTC.After(w.End) {
				return true, nil
			}
		}
	}

	if lastErr != nil {
		return false, fmt.Errorf("availability config fetch failed: %w", lastErr)
	}
	return false, nil
}

func windowsFromConfig(cfg scheduling.AvailabilityConfig) []availability.Interval {
	if len(cfg.WindowsUTC) > 0 {
		out := make([]availability.Interval, 0, len(cfg.WindowsUTC))
		for _, w := range cfg.WindowsUTC {
			start := w.StartUTC.UTC()
			end := w.EndUTC.UTC()
			if end.After(start) {
				out = append(out, availability.Interval{Start: start, End: end})
			}
		}
		return out
	}
	if cfg.WorkStartUTC.IsZero() || cfg.WorkEndUTC.IsZero() {
		return nil
	}
	if !cfg.WorkEndUTC.After(cfg.WorkStartUTC) {
		return nil
	}
	return []availability.Interval{{Start: cfg.WorkStartUTC.UTC(), End: cfg.WorkEndUTC.UTC()}}
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
