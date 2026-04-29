package grpcserver

import (
	"log/slog"

	"google.golang.org/grpc"
)

// New creates a gRPC server pre-wired with the given interceptors plus the
// built-in recovery, logger, and request-ID interceptors.
func New(log *slog.Logger, interceptors ...grpc.UnaryServerInterceptor) *grpc.Server {
	chain := make([]grpc.UnaryServerInterceptor, 0, len(interceptors)+3)
	chain = append(chain,
		RecoveryInterceptor(log),
		RequestIDInterceptor(),
		LoggerInterceptor(log),
	)
	chain = append(chain, interceptors...)

	return grpc.NewServer(grpc.ChainUnaryInterceptor(chain...))
}
