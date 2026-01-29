package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/md-rashed-zaman/apptremind/services/business-service/internal/storage"
)

type Handler struct {
	repo *storage.Repository
}

func New(repo *storage.Repository) *Handler {
	return &Handler{repo: repo}
}

func businessIDFromHeader(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Business-Id"))
}

func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := businessIDFromHeader(r)
	if businessID == "" {
		http.Error(w, "missing X-Business-Id", http.StatusBadRequest)
		return
	}

	p, err := h.repo.GetOrCreateProfile(r.Context(), businessID)
	if err != nil {
		http.Error(w, "failed to load profile", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"business_id":              p.BusinessID,
		"name":                     p.Name,
		"timezone":                 p.Timezone,
		"reminder_offsets_minutes": p.OffsetsMins,
	})
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := businessIDFromHeader(r)
	if businessID == "" {
		http.Error(w, "missing X-Business-Id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name                   string `json:"name"`
		Timezone               string `json:"timezone"`
		ReminderOffsetsMinutes []int  `json:"reminder_offsets_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Timezone = strings.TrimSpace(req.Timezone)
	if req.Timezone == "" {
		req.Timezone = "UTC"
	}

	var offsets []int
	for _, v := range req.ReminderOffsetsMinutes {
		if v <= 0 || v > 365*24*60 {
			http.Error(w, "invalid reminder_offsets_minutes", http.StatusBadRequest)
			return
		}
		offsets = append(offsets, v)
	}
	if len(offsets) == 0 {
		offsets = []int{1440, 60}
	}

	if err := h.repo.UpdateProfile(r.Context(), businessID, req.Name, req.Timezone, offsets); err != nil {
		http.Error(w, "failed to update profile", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := businessIDFromHeader(r)
	if businessID == "" {
		http.Error(w, "missing X-Business-Id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name         string  `json:"name"`
		DurationMins int     `json:"duration_minutes"`
		Price        float64 `json:"price"`
		Description  string  `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" || req.DurationMins <= 0 {
		http.Error(w, "name and duration_minutes required", http.StatusBadRequest)
		return
	}

	id, err := h.repo.CreateService(r.Context(), businessID, req.Name, req.DurationMins, strconv.FormatFloat(req.Price, 'f', 2, 64), req.Description)
	if err != nil {
		http.Error(w, "failed to create service", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": id,
	})
}

func (h *Handler) ListServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := businessIDFromHeader(r)
	if businessID == "" {
		http.Error(w, "missing X-Business-Id", http.StatusBadRequest)
		return
	}

	services, err := h.repo.ListServices(r.Context(), businessID, 100)
	if err != nil {
		http.Error(w, "failed to list services", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(services)
}

func (h *Handler) CreateStaff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := businessIDFromHeader(r)
	if businessID == "" {
		http.Error(w, "missing X-Business-Id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name     string `json:"name"`
		IsActive *bool  `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	id, err := h.repo.CreateStaff(r.Context(), businessID, req.Name, isActive)
	if err != nil {
		http.Error(w, "failed to create staff", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": id,
	})
}

func (h *Handler) ListStaff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := businessIDFromHeader(r)
	if businessID == "" {
		http.Error(w, "missing X-Business-Id", http.StatusBadRequest)
		return
	}

	staff, err := h.repo.ListStaff(r.Context(), businessID, 100)
	if err != nil {
		http.Error(w, "failed to list staff", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(staff)
}

func (h *Handler) ListWorkingHours(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := businessIDFromHeader(r)
	if businessID == "" {
		http.Error(w, "missing X-Business-Id", http.StatusBadRequest)
		return
	}

	staffID := strings.TrimSpace(r.URL.Query().Get("staff_id"))
	if staffID == "" {
		http.Error(w, "staff_id is required", http.StatusBadRequest)
		return
	}

	wh, err := h.repo.ListWorkingHours(r.Context(), businessID, staffID)
	if err != nil {
		http.Error(w, "failed to list working hours", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(wh)
}

func (h *Handler) UpsertWorkingHours(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := businessIDFromHeader(r)
	if businessID == "" {
		http.Error(w, "missing X-Business-Id", http.StatusBadRequest)
		return
	}

	staffID := strings.TrimSpace(r.URL.Query().Get("staff_id"))
	if staffID == "" {
		http.Error(w, "staff_id is required", http.StatusBadRequest)
		return
	}

	var req struct {
		Weekday     int  `json:"weekday"`
		IsWorking   bool `json:"is_working"`
		StartMinute int  `json:"start_minute"`
		EndMinute   int  `json:"end_minute"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	if req.Weekday < 0 || req.Weekday > 6 {
		http.Error(w, "weekday must be between 0 and 6", http.StatusBadRequest)
		return
	}

	startMin := req.StartMinute
	endMin := req.EndMinute
	if !req.IsWorking {
		startMin = 0
		endMin = 0
	} else {
		if startMin < 0 || startMin >= 1440 || endMin <= 0 || endMin > 1440 || startMin >= endMin {
			http.Error(w, "invalid start_minute/end_minute", http.StatusBadRequest)
			return
		}
	}

	if err := h.repo.UpsertWorkingHours(r.Context(), businessID, staffID, req.Weekday, req.IsWorking, startMin, endMin); err != nil {
		http.Error(w, "failed to upsert working hours", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateTimeOff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := businessIDFromHeader(r)
	if businessID == "" {
		http.Error(w, "missing X-Business-Id", http.StatusBadRequest)
		return
	}

	staffID := strings.TrimSpace(r.URL.Query().Get("staff_id"))
	if staffID == "" {
		http.Error(w, "staff_id is required", http.StatusBadRequest)
		return
	}

	var req struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)

	start, err := time.Parse(time.RFC3339, strings.TrimSpace(req.StartTime))
	if err != nil {
		http.Error(w, "invalid start_time", http.StatusBadRequest)
		return
	}
	end, err := time.Parse(time.RFC3339, strings.TrimSpace(req.EndTime))
	if err != nil {
		http.Error(w, "invalid end_time", http.StatusBadRequest)
		return
	}
	if !end.After(start) {
		http.Error(w, "end_time must be after start_time", http.StatusBadRequest)
		return
	}

	id, err := h.repo.CreateTimeOff(r.Context(), businessID, staffID, start.UTC(), end.UTC(), req.Reason)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			http.Error(w, "time off overlaps existing entry", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create time off", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"id": id})
}

func (h *Handler) ListTimeOff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := businessIDFromHeader(r)
	if businessID == "" {
		http.Error(w, "missing X-Business-Id", http.StatusBadRequest)
		return
	}

	staffID := strings.TrimSpace(r.URL.Query().Get("staff_id"))
	if staffID == "" {
		http.Error(w, "staff_id is required", http.StatusBadRequest)
		return
	}

	fromStr := strings.TrimSpace(r.URL.Query().Get("from"))
	toStr := strings.TrimSpace(r.URL.Query().Get("to"))
	if fromStr == "" || toStr == "" {
		http.Error(w, "from and to are required (RFC3339)", http.StatusBadRequest)
		return
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		http.Error(w, "invalid from", http.StatusBadRequest)
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		http.Error(w, "invalid to", http.StatusBadRequest)
		return
	}
	if !to.After(from) {
		http.Error(w, "to must be after from", http.StatusBadRequest)
		return
	}

	items, err := h.repo.ListTimeOff(r.Context(), businessID, staffID, from.UTC(), to.UTC(), 100)
	if err != nil {
		http.Error(w, "failed to list time off", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(items)
}

func (h *Handler) DeleteTimeOff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	businessID := businessIDFromHeader(r)
	if businessID == "" {
		http.Error(w, "missing X-Business-Id", http.StatusBadRequest)
		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	if err := h.repo.DeleteTimeOff(r.Context(), businessID, id); err != nil {
		http.Error(w, "failed to delete time off", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
