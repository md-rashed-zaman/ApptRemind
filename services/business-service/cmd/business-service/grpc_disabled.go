//go:build !protogen

package main

import (
	"context"
	"log/slog"

	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/services/business-service/internal/storage"
)

func startGrpcServer(_ context.Context, _ *slog.Logger, _ *db.Pool, _ *storage.Repository) error {
	return nil
}
