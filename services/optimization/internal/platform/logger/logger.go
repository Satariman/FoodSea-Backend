package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// New builds an slog.Logger for the given environment.
// "production" → JSON/Info, "test" → discarded, default → Text/Debug.
func New(env string) *slog.Logger {
	opts := &slog.HandlerOptions{}
	var handler slog.Handler

	switch env {
	case "production":
		opts.Level = slog.LevelInfo
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "test":
		opts.Level = slog.LevelError
		handler = slog.NewTextHandler(io.Discard, opts)
	default:
		opts.Level = slog.LevelDebug
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(&ContextHandler{Handler: handler})
}

// ContextHandler wraps a slog.Handler and injects request_id from context.
type ContextHandler struct {
	slog.Handler
}

//nolint:gocritic // slog.Handler interface requires passing slog.Record by value.
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := ctx.Value(requestIDKey).(string); ok && id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

// WithRequestID stores a request ID into the context for later retrieval by the logger.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext returns the request ID stored in the context.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey).(string)
	return id, ok
}

// FromContext returns a logger with request_id already bound (no-op if absent).
func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	if id, ok := RequestIDFromContext(ctx); ok {
		return base.With("request_id", id)
	}
	return base
}
