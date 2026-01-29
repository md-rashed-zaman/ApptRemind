package kafkax

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"github.com/segmentio/kafka-go"
)

// InjectTraceHeaders appends W3C trace context headers to Kafka headers.
func InjectTraceHeaders(ctx context.Context, headers []kafka.Header) []kafka.Header {
	carrier := kafkaHeaderCarrier{headers: headers}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier.headers
}

// ExtractTraceContext returns a context extracted from Kafka headers using the global propagator.
func ExtractTraceContext(ctx context.Context, msg kafka.Message) context.Context {
	carrier := kafkaHeaderCarrier{headers: msg.Headers}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

type kafkaHeaderCarrier struct {
	headers []kafka.Header
}

func (c kafkaHeaderCarrier) Get(key string) string {
	for _, h := range c.headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c kafkaHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for _, h := range c.headers {
		keys = append(keys, h.Key)
	}
	return keys
}

func (c kafkaHeaderCarrier) Set(key string, value string) {
	// Overwrite existing key if present to avoid duplicates.
	for i := range c.headers {
		if c.headers[i].Key == key {
			c.headers[i].Value = []byte(value)
			return
		}
	}
	c.headers = append(c.headers, kafka.Header{Key: key, Value: []byte(value)})
}

var _ propagation.TextMapCarrier = kafkaHeaderCarrier{}

