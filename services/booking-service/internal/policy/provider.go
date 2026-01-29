package policy

import (
	"context"
	"time"
)

type Provider interface {
	ReminderOffsets(ctx context.Context, businessID string) ([]time.Duration, error)
}

type staticProvider struct {
	offsets []time.Duration
}

func NewStaticProvider(offsets []time.Duration) Provider {
	return &staticProvider{offsets: offsets}
}

func (p *staticProvider) ReminderOffsets(_ context.Context, _ string) ([]time.Duration, error) {
	return p.offsets, nil
}
