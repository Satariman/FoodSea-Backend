package logger_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/foodsea/core/internal/platform/logger"
)

func TestNew_Development(t *testing.T) {
	l := logger.New("development")
	assert.NotNil(t, l)
	assert.True(t, l.Enabled(context.Background(), slog.LevelDebug))
}

func TestNew_Production(t *testing.T) {
	l := logger.New("production")
	assert.NotNil(t, l)
	assert.False(t, l.Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, l.Enabled(context.Background(), slog.LevelInfo))
}

func TestNew_Test(t *testing.T) {
	l := logger.New("test")
	assert.NotNil(t, l)
	assert.False(t, l.Enabled(context.Background(), slog.LevelWarn))
}

func TestContextHandler_InjectsRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	base := slog.New(&logger.ContextHandler{Handler: slog.NewTextHandler(buf, opts)})

	ctx := logger.WithRequestID(context.Background(), "test-req-id")
	base.InfoContext(ctx, "hello")

	assert.True(t, strings.Contains(buf.String(), "test-req-id"), "request_id should appear in log output")
}

func TestContextHandler_NoRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	base := slog.New(&logger.ContextHandler{Handler: slog.NewTextHandler(buf, opts)})

	base.InfoContext(context.Background(), "hello")
	assert.False(t, strings.Contains(buf.String(), "request_id"))
}

func TestFromContext_WithID(t *testing.T) {
	l := logger.New("test")
	ctx := logger.WithRequestID(context.Background(), "abc")
	result := logger.FromContext(ctx, l)
	assert.NotNil(t, result)
}
