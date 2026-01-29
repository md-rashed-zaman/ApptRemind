package kafkax

import (
	"context"
	"errors"
	"time"

	"github.com/segmentio/kafka-go"
)

func ReadyCheck(brokers string) func(context.Context) error {
	return func(ctx context.Context) error {
		list := SplitBrokers(brokers)
		if len(list) == 0 {
			return errors.New("kafka brokers not configured")
		}
		dialer := kafka.Dialer{Timeout: 2 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", list[0])
		if err != nil {
			return err
		}
		_ = conn.Close()
		return nil
	}
}
