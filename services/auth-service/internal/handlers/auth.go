package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/md-rashed-zaman/apptremind/libs/auth"
	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/services/auth-service/internal/audit"
	"github.com/md-rashed-zaman/apptremind/services/auth-service/internal/outbox"
	"github.com/md-rashed-zaman/apptremind/services/auth-service/internal/sessions"
	"github.com/md-rashed-zaman/apptremind/services/auth-service/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	signer       TokenSigner
	pool         *db.Pool
	users        *storage.UserRepository
	audit        *audit.Repository
	outbox       *outbox.Repository
	refreshRepo  *sessions.RefreshRepository
	refreshToken time.Duration
}

func NewAuthHandler(
	signer TokenSigner,
	pool *db.Pool,
	users *storage.UserRepository,
	auditRepo *audit.Repository,
	outboxRepo *outbox.Repository,
	refreshRepo *sessions.RefreshRepository,
	refreshTTL time.Duration,
) *AuthHandler {
	return &AuthHandler{
		signer:       signer,
		pool:         pool,
		users:        users,
		audit:        auditRepo,
		outbox:       outboxRepo,
		refreshRepo:  refreshRepo,
		refreshToken: refreshTTL,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	BusinessName string `json:"business_name"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type meResponse struct {
	UserID     string `json:"user_id"`
	BusinessID string `json:"business_id"`
	Role       string `json:"role"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)
	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	user := storage.User{
		ID:           uuid.NewString(),
		BusinessID:   uuid.NewString(),
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         "owner",
	}
	ctx := r.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		http.Error(w, "failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := h.users.CreateTx(ctx, tx, user); err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			http.Error(w, "email already registered", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	createdPayload, err := json.Marshal(map[string]any{
		"user_id":     user.ID,
		"business_id": user.BusinessID,
		"email":       user.Email,
		"role":        user.Role,
		"created_at":  time.Now().UTC(),
	})
	if err != nil {
		http.Error(w, "failed to marshal user event", http.StatusInternalServerError)
		return
	}
	if err := h.outbox.Insert(ctx, tx, outbox.Event{
		AggregateType: "user",
		AggregateID:   user.ID,
		EventType:     "auth.user.created.v1",
		Payload:       createdPayload,
	}); err != nil {
		http.Error(w, "failed to enqueue user event", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "failed to commit transaction", http.StatusInternalServerError)
		return
	}

	token, err := issueJWT(user, h.signer)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}
	refreshToken, err := h.issueRefreshToken(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "failed to issue refresh token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(loginResponse{
		AccessToken:  token,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") || len(strings.TrimSpace(authHeader)) <= len("Bearer ") {
		http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	claims, err := h.signer.Verify(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(meResponse{
		UserID:     claims.Sub,
		BusinessID: claims.BusinessID,
		Role:       claims.Role,
	})
}

func (h *AuthHandler) JWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jwks := h.signer.JWKS()
	if len(jwks) == 0 {
		http.Error(w, "jwks not available", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"keys": jwks,
	})
}

func (h *AuthHandler) Rotate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.signer.CanRotate() {
		http.Error(w, "rotation not enabled", http.StatusBadRequest)
		return
	}

	reqKey := r.Header.Get("X-Rotate-Key")
	if reqKey == "" || reqKey != h.signer.RotateKey() {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ActiveKid string `json:"active_kid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	if req.ActiveKid == "" {
		http.Error(w, "active_kid is required", http.StatusBadRequest)
		return
	}

	if err := h.signer.SetActiveKid(req.ActiveKid); err != nil {
		http.Error(w, "invalid active_kid", http.StatusBadRequest)
		return
	}

	if h.audit != nil {
		_ = h.audit.RecordWithOutbox(r.Context(), h.outbox, "jwt.rotate", "", map[string]any{
			"active_kid": req.ActiveKid,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Audit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.audit == nil {
		http.Error(w, "audit not available", http.StatusNotFound)
		return
	}

	reqKey := r.Header.Get("X-Rotate-Key")
	if reqKey == "" || reqKey != h.signer.RotateKey() {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	events, err := h.audit.ListRecent(r.Context(), limit)
	if err != nil {
		http.Error(w, "failed to load audit events", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(events)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)
	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}

	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if storage.IsNotFound(err) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		http.Error(w, "failed to lookup user", http.StatusInternalServerError)
		return
	}

	if err := verifyPassword(user.PasswordHash, req.Password); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := issueJWT(user, h.signer)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}
	refreshToken, err := h.issueRefreshToken(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "failed to issue refresh token", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(loginResponse{
		AccessToken:  token,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		http.Error(w, "refresh_token required", http.StatusBadRequest)
		return
	}

	hash := sessions.HashToken(req.RefreshToken)
	tokenRecord, err := h.refreshRepo.GetByHash(r.Context(), hash)
	if err != nil {
		if sessions.IsNotFound(err) {
			http.Error(w, "invalid refresh token", http.StatusUnauthorized)
			return
		}
		http.Error(w, "failed to lookup refresh token", http.StatusInternalServerError)
		return
	}
	if tokenRecord.RevokedAt != nil || tokenRecord.ExpiresAt.Before(time.Now()) {
		http.Error(w, "refresh token expired", http.StatusUnauthorized)
		return
	}

	user, err := h.users.GetByID(r.Context(), tokenRecord.UserID)
	if err != nil {
		if storage.IsNotFound(err) {
			http.Error(w, "invalid refresh token", http.StatusUnauthorized)
			return
		}
		http.Error(w, "failed to lookup user", http.StatusInternalServerError)
		return
	}

	if err := h.refreshRepo.Revoke(r.Context(), tokenRecord.ID); err != nil {
		http.Error(w, "failed to rotate refresh token", http.StatusInternalServerError)
		return
	}

	newRefreshToken, err := h.issueRefreshToken(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "failed to issue refresh token", http.StatusInternalServerError)
		return
	}

	newAccessToken, err := issueJWT(user, h.signer)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(loginResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		http.Error(w, "refresh_token required", http.StatusBadRequest)
		return
	}

	hash := sessions.HashToken(req.RefreshToken)
	tokenRecord, err := h.refreshRepo.GetByHash(r.Context(), hash)
	if err != nil {
		if sessions.IsNotFound(err) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "failed to lookup refresh token", http.StatusInternalServerError)
		return
	}

	if tokenRecord.RevokedAt == nil {
		if err := h.refreshRepo.Revoke(r.Context(), tokenRecord.ID); err != nil {
			http.Error(w, "failed to revoke refresh token", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func issueJWT(user storage.User, signer TokenSigner) (string, error) {
	now := time.Now().Unix()
	return signer.Sign(auth.Claims{
		Sub:        user.ID,
		BusinessID: user.BusinessID,
		Role:       user.Role,
		Iat:        now,
		Exp:        time.Now().Add(1 * time.Hour).Unix(),
	})
}

func (h *AuthHandler) issueRefreshToken(ctx context.Context, userID string) (string, error) {
	raw, err := newRefreshToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(h.refreshToken)
	if _, err := h.refreshRepo.Create(ctx, userID, raw, expiresAt); err != nil {
		return "", err
	}
	return raw, nil
}

func newRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashPassword(raw string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyPassword(hash string, raw string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw))
}
