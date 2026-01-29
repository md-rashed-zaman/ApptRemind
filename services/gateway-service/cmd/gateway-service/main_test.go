package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/auth"
)

func TestRequireRole(t *testing.T) {
	h := requireRole(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "owner", "admin")

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("X-Role", "member")
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rw.Code)
	}

	reqOK := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	reqOK.Header.Set("X-Role", "owner")
	rwOK := httptest.NewRecorder()
	h.ServeHTTP(rwOK, reqOK)
	if rwOK.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rwOK.Code)
	}
}

func TestRequireAuthHS256(t *testing.T) {
	secret := "test-secret"
	claims := auth.Claims{
		Sub:        "user-1",
		BusinessID: "biz-1",
		Role:       "owner",
		Iat:        time.Now().Unix(),
		Exp:        time.Now().Add(1 * time.Hour).Unix(),
	}
	token, err := auth.SignHS256(claims, secret)
	if err != nil {
		t.Fatalf("SignHS256 failed: %v", err)
	}

	h := requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User-Id") != claims.Sub {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("X-Business-Id") != claims.BusinessID {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("X-Role") != claims.Role {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}), secret, nil)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rw.Code)
	}

	reqBad := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	reqBad.Header.Set("Authorization", "Bearer badtoken")
	rwBad := httptest.NewRecorder()
	h.ServeHTTP(rwBad, reqBad)
	if rwBad.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rwBad.Code)
	}
}
