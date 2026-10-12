package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/helper"
)

const requestIDHeader = "X-Request-ID"

// RequestID gives every request a UUID (spec "Logging & Observability").
// A client-supplied X-Request-ID is reused only if it is a valid UUID.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.NewString()
		}
		c.Set(helper.CtxRequestID, id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}
