package grpcserver

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type grpcRequestIDKey struct{}

// RecoveryInterceptor catches panics and returns codes.Internal.
func RecoveryInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorContext(ctx, "grpc panic recovered",
					"error", r,
					"method", info.FullMethod,
					"stack", string(debug.Stack()),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// RequestIDInterceptor extracts x-request-id from gRPC metadata or generates a new one.
func RequestIDInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		reqID := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("x-request-id"); len(vals) > 0 {
				reqID = vals[0]
			}
		}
		if reqID == "" {
			reqID = uuid.New().String()
		}
		ctx = context.WithValue(ctx, grpcRequestIDKey{}, reqID)
		return handler(ctx, req)
	}
}

// LoggerInterceptor logs each gRPC call with method, status code, and duration.
func LoggerInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		reqID, _ := ctx.Value(grpcRequestIDKey{}).(string)
		log.InfoContext(ctx, "grpc request",
			"method", info.FullMethod,
			"code", code.String(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", reqID,
		)
		return resp, err
	}
}
