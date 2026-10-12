package middleware

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/Fadhelbulloh/task-board-api/internal/helper"
)

// Recovery turns a panic into a normal 500 response. The stack trace is
// logged, never returned to the client.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered",
					"request_id", helper.GetRequestID(c),
					"panic", fmt.Sprint(r),
					"stack", string(debug.Stack()),
				)
				helper.RespondError(c, helper.ErrInternal(fmt.Errorf("panic: %v", r)))
			}
		}()
		c.Next()
	}
}
