package middleware

// contextKey is a private type to avoid collisions with other packages using context.
type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
)
