package grpcclient

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb_core "github.com/foodsea/proto/core"
	pb_opt "github.com/foodsea/proto/optimization"

	"github.com/foodsea/ordering/internal/platform/config"
)

// ClientSet holds gRPC clients to upstream services.
type ClientSet struct {
	Cart         pb_core.CartServiceClient
	Optimization pb_opt.OptimizationServiceClient
	closers      []io.Closer
}

type connCloser struct{ conn *grpc.ClientConn }

func (c *connCloser) Close() error { return c.conn.Close() }

// Dial establishes gRPC connections to core and optimization services.
func Dial(ctx context.Context, cfg config.Config, log *slog.Logger) (*ClientSet, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithChainUnaryInterceptor(
			requestIDInterceptor(),
			retryInterceptor(3, log),
			loggingInterceptor(log),
		),
	}

	coreConn, err := grpc.NewClient(cfg.GRPCClients.CoreAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("dialing core gRPC: %w", err)
	}
	log.InfoContext(ctx, "grpc client connected", "service", "core", "addr", cfg.GRPCClients.CoreAddr)

	optConn, err := grpc.NewClient(cfg.GRPCClients.OptimizationAddr, opts...)
	if err != nil {
		_ = coreConn.Close()
		return nil, fmt.Errorf("dialing optimization gRPC: %w", err)
	}
	log.InfoContext(ctx, "grpc client connected", "service", "optimization", "addr", cfg.GRPCClients.OptimizationAddr)

	return &ClientSet{
		Cart:         pb_core.NewCartServiceClient(coreConn),
		Optimization: pb_opt.NewOptimizationServiceClient(optConn),
		closers:      []io.Closer{&connCloser{coreConn}, &connCloser{optConn}},
	}, nil
}

// Close shuts down all underlying gRPC connections.
func (c *ClientSet) Close() error {
	var errs []error
	for _, cl := range c.closers {
		if err := cl.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("closing grpc clients: %v", errs)
	}
	return nil
}

// requestIDInterceptor propagates x-request-id from context into outgoing metadata.
func requestIDInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if reqID, ok := ctx.Value(requestIDContextKey{}).(string); ok && reqID != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "x-request-id", reqID)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// retryInterceptor retries on codes.Unavailable with exponential backoff.
func retryInterceptor(maxAttempts int, log *slog.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		backoff := 100 * time.Millisecond
		var lastErr error
		for attempt := 0; attempt < maxAttempts; attempt++ {
			lastErr = invoker(ctx, method, req, reply, cc, opts...)
			if lastErr == nil {
				return nil
			}
			if status.Code(lastErr) != codes.Unavailable {
				return lastErr
			}
			if attempt < maxAttempts-1 {
				log.WarnContext(ctx, "grpc unavailable, retrying",
					"method", method,
					"attempt", attempt+1,
					"backoff", backoff,
				)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= 2
				if backoff > time.Second {
					backoff = time.Second
				}
			}
		}
		return lastErr
	}
}

// loggingInterceptor logs method, status code and duration for each outgoing call.
func loggingInterceptor(log *slog.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		log.InfoContext(ctx, "grpc client call",
			"method", method,
			"code", code.String(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return err
	}
}

// requestIDContextKey is the key used to store request IDs in outgoing contexts.
// Must match the key used by middleware.RequestIDFromContext.
type requestIDContextKey struct{}

// WithRequestID attaches a request ID to the context for gRPC propagation.
// Call this from HTTP middleware before making outgoing gRPC calls.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, id)
}
