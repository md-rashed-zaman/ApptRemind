//go:build protogen

package policy

import (
	"context"
	"log/slog"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/grpcx"
	businessv1 "github.com/md-rashed-zaman/apptremind/protos/gen/business/v1"
)

type grpcProvider struct {
	client businessv1.BusinessServiceClient
}

func NewBusinessPolicyProvider(logger *slog.Logger, fallback []time.Duration, addr string) (Provider, error) {
	if addr == "" {
		return NewStaticProvider(fallback), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpcx.Dial(ctx, addr, grpcx.DialOptions{Timeout: 5 * time.Second})
	if err != nil {
		logger.Warn("grpc policy provider unavailable, using fallback", "err", err)
		return NewStaticProvider(fallback), nil
	}

	logger.Info("grpc policy provider enabled", "addr", addr)
	return &grpcProvider{client: businessv1.NewBusinessServiceClient(conn)}, nil
}

func (p *grpcProvider) ReminderOffsets(ctx context.Context, businessID string) ([]time.Duration, error) {
	resp, err := p.client.GetBusinessProfile(ctx, &businessv1.BusinessProfileRequest{BusinessId: businessID})
	if err != nil {
		return nil, err
	}
	var offsets []time.Duration
	for _, mins := range resp.GetReminderPolicy().GetReminderOffsetsMinutes() {
		if mins <= 0 {
			continue
		}
		offsets = append(offsets, time.Duration(mins)*time.Minute)
	}
	return offsets, nil
}
