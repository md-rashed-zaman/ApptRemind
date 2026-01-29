package runtime

import (
	"log/slog"
	"os"
)

func NewLogger(service string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(h).With("service", service)
}

