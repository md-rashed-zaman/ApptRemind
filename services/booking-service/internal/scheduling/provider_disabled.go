//go:build !protogen

package scheduling

import (
	"context"
	"time"
)

type AvailabilityConfig struct {
	IsWorking       bool
	WorkStartUTC    time.Time
	WorkEndUTC      time.Time
	WindowsUTC      []AvailabilityWindow
	DurationMinutes int
	SlotStepMinutes int
	Timezone        string
}

type AvailabilityWindow struct {
	StartUTC time.Time
	EndUTC   time.Time
}

type Provider interface {
	GetAvailabilityConfig(ctx context.Context, businessID, staffID, serviceID string, date string) (AvailabilityConfig, error)
}

func NewProvider(_ string) (Provider, error) {
	return nil, nil
}
