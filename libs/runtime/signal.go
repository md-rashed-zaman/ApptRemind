package runtime

import (
	"context"
	"os/signal"
	"syscall"
)

func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

