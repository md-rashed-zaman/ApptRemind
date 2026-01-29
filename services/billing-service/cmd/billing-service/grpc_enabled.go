//go:build protogen

package main

import (
	"context"
	"log/slog"
	"net"

	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/libs/grpcx"
	"github.com/md-rashed-zaman/apptremind/libs/runtime"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/entitlements"
	"github.com/md-rashed-zaman/apptremind/services/billing-service/internal/storage"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func startGrpcServer(ctx context.Context, logger *slog.Logger, pool *db.Pool) error {
	port := runtime.Getenv("GRPC_PORT", "9091")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}

	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(grpcx.UnaryServerRequestIDInterceptor()),
	)
	entitlements.Register(srv, storage.NewRepository(pool))

	go func() {
		logger.Info("grpc server starting", "addr", lis.Addr().String())
		if err := srv.Serve(lis); err != nil {
			logger.Error("grpc server error", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()
		srv.GracefulStop()
	}()

	return nil
}
