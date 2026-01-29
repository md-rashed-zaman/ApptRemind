//go:build protogen

package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/runtime"
	"github.com/md-rashed-zaman/apptremind/services/booking-service/internal/entitlements"
)

func setupEntitlementsRoutes(ctx context.Context, mux *http.ServeMux, logger *slog.Logger) {
	addr := runtime.Getenv("BILLING_GRPC_ADDR", "billing-service:9091")
	client, err := entitlements.NewClient(addr)
	if err != nil {
		logger.Error("entitlements client init failed", "err", err)
		return
	}

	go func() {
		<-ctx.Done()
		_ = client.Close()
	}()

	mux.HandleFunc("/debug/entitlements", func(w http.ResponseWriter, r *http.Request) {
		businessID := r.URL.Query().Get("business_id")
		if businessID == "" {
			http.Error(w, "business_id is required", http.StatusBadRequest)
			return
		}

		reqCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		resp, err := client.GetEntitlements(reqCtx, businessID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

