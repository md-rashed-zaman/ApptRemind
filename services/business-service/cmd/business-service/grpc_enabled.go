//go:build protogen

package main

import (
	"context"
	"net"

	"github.com/md-rashed-zaman/apptremind/libs/config"
	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/libs/grpcx"
	"log/slog"

	"github.com/md-rashed-zaman/apptremind/services/business-service/internal/grpcserver"
	"github.com/md-rashed-zaman/apptremind/services/business-service/internal/storage"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func startGrpcServer(ctx context.Context, logger *slog.Logger, pool *db.Pool, repo *storage.Repository) error {
	port, err := config.Port("GRPC_PORT", "9090")
	if err != nil {
		return err
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}

	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(grpcx.UnaryServerRequestIDInterceptor()),
	)
	grpcserver.Register(srv, pool, repo)

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
