package platform

import (
	"context"
	"fmt"
	"net"
	"strconv"
)

// PortChecker verifies that a new listener port can be bound before persisting it.
type PortChecker struct{}

func (PortChecker) Check(ctx context.Context, port int) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", net.JoinHostPort("", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", port, err)
	}
	return listener.Close()
}
