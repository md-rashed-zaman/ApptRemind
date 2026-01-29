package grpcx

import (
	"context"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type DialOptions struct {
	Timeout time.Duration
	// If nil, defaults to insecure credentials (suitable for local dev / inside a cluster with mTLS at mesh layer).
	TransportCredentials grpc.DialOption
}

func Dial(ctx context.Context, addr string, opts DialOptions, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 3 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	dialOpts := []grpc.DialOption{
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(UnaryClientRequestIDInterceptor()),
		grpc.WithBlock(),
	}
	if opts.TransportCredentials != nil {
		dialOpts = append(dialOpts, opts.TransportCredentials)
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	dialOpts = append(dialOpts, extra...)

	return grpc.DialContext(ctx, addr, dialOpts...)
}
