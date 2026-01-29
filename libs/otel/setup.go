package otelx

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

type Config struct {
	Enabled      bool
	ServiceName  string
	OTLPEndpoint string // host:port, e.g. jaeger:4317
	SampleRatio  float64
}

func ConfigFromEnv(serviceName string) Config {
	enabled := true
	if v := strings.TrimSpace(getenv("OTEL_ENABLED", "true")); v != "" {
		enabled = v != "false" && v != "0"
	}

	endpoint := strings.TrimSpace(getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "jaeger:4317"))

	sampleRatio := 1.0
	if v := strings.TrimSpace(getenv("OTEL_SAMPLING_RATIO", "1")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			sampleRatio = f
		}
	}

	return Config{
		Enabled:      enabled,
		ServiceName:  serviceName,
		OTLPEndpoint: endpoint,
		SampleRatio:  sampleRatio,
	}
}

// Setup configures a global tracer provider + propagators.
// Call the returned shutdown func during graceful shutdown.
func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithTimeout(3*time.Second),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

func getenv(key, fallback string) string {
	if v, ok := lookupEnv(key); ok {
		return v
	}
	return fallback
}

