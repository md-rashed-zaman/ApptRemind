package outbox

type Event struct {
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
}
