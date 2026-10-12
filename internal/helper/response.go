package helper

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Fadhelbulloh/task-board-api/internal/dto"
)

// RespondError writes the standard error body and aborts the chain. Every
// error path (controller, middleware, panic recovery) goes through here so
// the shape never drifts.
func RespondError(c *gin.Context, err error) {
	appErr := AsAppError(err)
	if appErr.Status >= 500 {
		slog.Error("server error",
			"request_id", GetRequestID(c),
			"code", appErr.Code,
			"error", appErr.Err,
		)
	}
	c.AbortWithStatusJSON(appErr.Status, dto.ErrorResponse{
		Status:    appErr.Status,
		Code:      appErr.Code,
		Message:   appErr.Message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: GetRequestID(c),
	})
}
