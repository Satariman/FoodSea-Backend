package httputil

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ParseUUID parses a path or query parameter as a UUID.
// If parsing fails it writes a BadRequest response and returns (uuid.Nil, false).
func ParseUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	raw := c.Param(name)
	if raw == "" {
		raw = c.Query(name)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		BadRequest(c, "invalid "+name+" format: must be a UUID")
		return uuid.Nil, false
	}
	return id, true
}
