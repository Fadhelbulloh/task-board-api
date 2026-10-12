package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Fadhelbulloh/task-board-api/internal/helper"
)

// Auth requires a valid "Authorization: Bearer <jwt>" header and puts the
// user id on the context (read it with helper.GetUserID).
func Auth(jwt *helper.JWT) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			helper.RespondError(c, helper.ErrUnauthorized("missing bearer token"))
			return
		}
		userID, err := jwt.Verify(token)
		if err != nil {
			helper.RespondError(c, helper.ErrUnauthorized("invalid or expired token"))
			return
		}
		c.Set(helper.CtxUserID, userID)
		c.Next()
	}
}
