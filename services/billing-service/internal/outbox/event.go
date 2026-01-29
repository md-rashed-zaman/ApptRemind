package outbox

// Event is the domain event envelope written to the outbox table.
// The Kafka topic name equals EventType (production-style: event per topic).
type Event struct {
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
}

