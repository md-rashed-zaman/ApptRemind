package kafkax

import (
	"strings"

	"github.com/segmentio/kafka-go"
)

// EventMeta is the canonical metadata carried on Kafka messages across services.
type EventMeta struct {
	EventID   string
	EventType string
}

func ExtractEventMeta(msg kafka.Message) EventMeta {
	eventID := HeaderValue(msg.Headers, "event_id")
	eventType := HeaderValue(msg.Headers, "event_type")
	if eventID == "" {
		eventID = string(msg.Key)
	}
	if eventType == "" {
		eventType = msg.Topic
	}
	return EventMeta{EventID: eventID, EventType: eventType}
}

func HeaderValue(headers []kafka.Header, key string) string {
	for _, h := range headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func SplitBrokers(raw string) []string {
	var brokers []string
	for _, b := range strings.Split(raw, ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			brokers = append(brokers, b)
		}
	}
	return brokers
}
