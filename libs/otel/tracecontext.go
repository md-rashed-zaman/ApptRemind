package otelx

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func TraceContextStrings(ctx context.Context) (traceparent string, tracestate string) {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier["traceparent"], carrier["tracestate"]
}

func ContextWithTraceContext(ctx context.Context, traceparent string, tracestate string) context.Context {
	if traceparent == "" && tracestate == "" {
		return ctx
	}
	carrier := propagation.MapCarrier{
		"traceparent": traceparent,
		"tracestate":  tracestate,
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

