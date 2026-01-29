//go:build !protogen

package policy

import (
	"log/slog"
	"time"
)

func NewBusinessPolicyProvider(_ *slog.Logger, offsets []time.Duration, _ string) (Provider, error) {
	return NewStaticProvider(offsets), nil
}
