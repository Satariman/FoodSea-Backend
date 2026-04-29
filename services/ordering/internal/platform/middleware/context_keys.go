package middleware

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
)
