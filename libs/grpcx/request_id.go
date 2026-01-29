package grpcx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
)

// RequestIDMetadataKey is the canonical key used for request id propagation over gRPC metadata.
// Lowercase is recommended by gRPC metadata conventions.
const RequestIDMetadataKey = "x-request-id"

func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

func NewRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
