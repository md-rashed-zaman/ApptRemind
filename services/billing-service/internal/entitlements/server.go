//go:build protogen

package entitlements

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	entitlementsv1 "github.com/md-rashed-zaman/apptremind/protos/gen/entitlements/v1"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/storage"
	"google.golang.org/grpc"
)

type server struct {
	entitlementsv1.UnimplementedEntitlementsServiceServer
	repo *storage.Repository
}

func Register(grpcServer *grpc.Server, repo *storage.Repository) {
	entitlementsv1.RegisterEntitlementsServiceServer(grpcServer, &server{repo: repo})
}

func (s *server) GetEntitlements(ctx context.Context, req *entitlementsv1.EntitlementsRequest) (*entitlementsv1.EntitlementsResponse, error) {
	limits := LimitsForTier("free")
	if s.repo != nil && req.GetBusinessId() != "" {
		sub, err := s.repo.GetSubscription(ctx, req.GetBusinessId())
		if err == nil && sub.Status == "active" {
			limits = LimitsForTier(sub.Tier)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			// keep response stable: treat repo errors as free tier
		}
	}
	return &entitlementsv1.EntitlementsResponse{
		Tier:                  limits.Tier,
		MaxStaff:              uint32(limits.MaxStaff),
		MaxServices:           uint32(limits.MaxServices),
		MaxMonthlyAppointments: uint32(limits.MaxMonthlyAppointments),
	}, nil
}
