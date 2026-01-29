//go:build protogen

package entitlements

import (
	"context"
	"net"
	"testing"
	"time"

	entitlementsv1 "github.com/md-rashed-zaman/apptremind/protos/gen/entitlements/v1"
	"google.golang.org/grpc"
)

type testServer struct {
	entitlementsv1.UnimplementedEntitlementsServiceServer
}

func (s *testServer) GetEntitlements(_ context.Context, _ *entitlementsv1.EntitlementsRequest) (*entitlementsv1.EntitlementsResponse, error) {
	return &entitlementsv1.EntitlementsResponse{
		Tier:                  "pro",
		MaxStaff:              10,
		MaxServices:           50,
		MaxMonthlyAppointments: 1000,
	}, nil
}

func TestClient_GetEntitlements(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()

	srv := grpc.NewServer()
	entitlementsv1.RegisterEntitlementsServiceServer(srv, &testServer{})

	go func() {
		_ = srv.Serve(lis)
	}()
	defer srv.Stop()

	client, err := NewClient(lis.Addr().String())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.GetEntitlements(ctx, "biz-123")
	if err != nil {
		t.Fatalf("get entitlements: %v", err)
	}
	if resp.Tier != "pro" {
		t.Fatalf("unexpected tier: %s", resp.Tier)
	}
}

