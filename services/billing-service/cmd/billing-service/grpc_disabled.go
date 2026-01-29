//go:build !protogen

package main

import (
	"context"
	"log/slog"

	"github.com/md-rashed-zaman/apptremind/libs/db"
)

func startGrpcServer(_ context.Context, _ *slog.Logger, _ *db.Pool) error {
	return nil
}
