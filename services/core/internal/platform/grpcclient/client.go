package grpcclient

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	pbml "github.com/foodsea/proto/ml"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

type ClientSet struct {
	Analog  pbml.AnalogServiceClient
	closers []io.Closer
}

type connCloser struct {
	conn *grpc.ClientConn
}

func (c *connCloser) Close() error {
	return c.conn.Close()
}

func DialML(ctx context.Context, addr string, log *slog.Logger, extraOpts ...grpc.DialOption) (*ClientSet, error) {
	opts := buildDialOptions(log, extraOpts...)
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("dialing ml gRPC: %w", err)
	}
	log.InfoContext(ctx, "grpc client connected", "service", "ml", "addr", addr)

	return &ClientSet{
		Analog:  pbml.NewAnalogServiceClient(conn),
		closers: []io.Closer{&connCloser{conn: conn}},
	}, nil
}

func (c *ClientSet) Close() error {
	if c == nil {
		return nil
	}

	var errs []error
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("closing grpc clients: %v", errs)
	}
	return nil
}

func buildDialOptions(log *slog.Logger, extraOpts ...grpc.DialOption) []grpc.DialOption {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithChainUnaryInterceptor(retryInterceptor(3, log), loggingInterceptor(log)),
	}
	return append(opts, extraOpts...)
}

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
				log.WarnContext(ctx, "grpc unavailable, retrying", "method", method, "attempt", attempt+1, "backoff", backoff)
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

func loggingInterceptor(log *slog.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		log.InfoContext(ctx, "grpc client call", "method", method, "code", code.String(), "duration_ms", time.Since(start).Milliseconds())
		return err
	}
}
