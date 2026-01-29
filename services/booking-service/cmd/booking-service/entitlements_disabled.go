//go:build !protogen

package main

import (
	"context"
	"log/slog"
	"net/http"
)

func setupEntitlementsRoutes(_ context.Context, _ *http.ServeMux, _ *slog.Logger) {}
