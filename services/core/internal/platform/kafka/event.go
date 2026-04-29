package kafka

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Event is the canonical envelope for all Kafka messages in FoodSea.
type Event struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	Source    string          `json:"source"`
	Payload   json.RawMessage `json:"payload"`
}

// NewEvent constructs an Event with a new UUID, current timestamp, and the
// provided payload marshalled to JSON.
func NewEvent(eventType, source string, payload any) (Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshaling payload for event %q: %w", eventType, err)
	}
	return Event{
		EventID:   uuid.New().String(),
		EventType: eventType,
		Timestamp: time.Now().UTC(),
		Source:    source,
		Payload:   raw,
	}, nil
}
