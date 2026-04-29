package grpcserver

import (
	"fmt"
	"net"
)

// Listen creates a TCP listener on the given port.
func Listen(port int) (net.Listener, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("grpc listen on port %d: %w", port, err)
	}
	return lis, nil
}
