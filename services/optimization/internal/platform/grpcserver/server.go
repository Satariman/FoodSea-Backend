package grpcserver

import (
	"log/slog"

	"google.golang.org/grpc"
)

// New creates a gRPC server pre-wired with recovery, request-ID, and logger interceptors.
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
