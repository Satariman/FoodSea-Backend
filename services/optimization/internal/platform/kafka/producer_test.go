package kafka

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProducer_ConfiguresWriterTopic(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"}, "optimization.events", slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer func() { _ = p.Close() }()

	assert.NotNil(t, p)
	assert.Equal(t, "optimization.events", p.writer.Topic)
}

func TestPublish_MarshalError(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"}, "optimization.events", slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer func() { _ = p.Close() }()

	e := Event{EventID: "1", EventType: "test", Payload: []byte("{")} // invalid json payload in envelope should still marshal as bytes
	err := p.Publish(context.Background(), e)
	assert.Error(t, err)
}
