package grpcx

import (
	"context"

	"github.com/md-rashed-zaman/apptremind/libs/httpx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// UnaryClientRequestIDInterceptor propagates request id from context into outgoing metadata.
//
// Priority:
// 1) httpx.RequestIDFromContext (HTTP -> gRPC fanout)
// 2) grpcx.RequestIDFromContext (gRPC -> gRPC chaining)
func UnaryClientRequestIDInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		id := httpx.RequestIDFromContext(ctx)
		if id == "" {
			id = RequestIDFromContext(ctx)
		}
		if id != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, RequestIDMetadataKey, id)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// UnaryServerRequestIDInterceptor reads request id from incoming metadata (if present),
// stores it in context, and echoes it back in response headers.
func UnaryServerRequestIDInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get(RequestIDMetadataKey); len(vals) > 0 {
				id = vals[0]
			}
		}
		if id == "" {
			id = NewRequestID()
		}
		_ = grpc.SetHeader(ctx, metadata.Pairs(RequestIDMetadataKey, id))
		ctx = WithRequestID(ctx, id)
		return handler(ctx, req)
	}
}
