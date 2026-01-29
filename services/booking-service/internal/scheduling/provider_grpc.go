//go:build protogen

package scheduling

import (
	"context"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/grpcx"
	businessv1 "github.com/md-rashed-zaman/apptremind/protos/gen/business/v1"
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

type grpcProvider struct {
	client businessv1.BusinessServiceClient
}

func NewProvider(addr string) (Provider, error) {
	if addr == "" {
		return nil, nil
	}
	conn, err := grpcx.Dial(context.Background(), addr, grpcx.DialOptions{Timeout: 3 * time.Second})
	if err != nil {
		return nil, err
	}
	return &grpcProvider{client: businessv1.NewBusinessServiceClient(conn)}, nil
}

func (p *grpcProvider) GetAvailabilityConfig(ctx context.Context, businessID, staffID, serviceID string, date string) (AvailabilityConfig, error) {
	resp, err := p.client.GetAvailabilityConfig(ctx, &businessv1.AvailabilityConfigRequest{
		BusinessId: businessID,
		StaffId:    staffID,
		ServiceId:  serviceID,
		Date:       date,
	})
	if err != nil {
		return AvailabilityConfig{}, err
	}
	cfg := AvailabilityConfig{
		IsWorking:       resp.GetIsWorking(),
		DurationMinutes: int(resp.GetDurationMinutes()),
		SlotStepMinutes: int(resp.GetSlotStepMinutes()),
		Timezone:        resp.GetTimezone(),
	}
	if resp.GetWorkStartUtc() != nil {
		cfg.WorkStartUTC = resp.GetWorkStartUtc().AsTime()
	}
	if resp.GetWorkEndUtc() != nil {
		cfg.WorkEndUTC = resp.GetWorkEndUtc().AsTime()
	}
	for _, w := range resp.GetWindowsUtc() {
		if w.GetStartUtc() == nil || w.GetEndUtc() == nil {
			continue
		}
		start := w.GetStartUtc().AsTime()
		end := w.GetEndUtc().AsTime()
		if end.After(start) {
			cfg.WindowsUTC = append(cfg.WindowsUTC, AvailabilityWindow{StartUTC: start, EndUTC: end})
		}
	}
	return cfg, nil
}
