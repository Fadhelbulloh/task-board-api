package helper

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Keys set by middleware and read by controllers.
const (
	CtxRequestID = "request_id"
	CtxUserID    = "user_id"
)

func GetRequestID(c *gin.Context) string {
	return c.GetString(CtxRequestID)
}

// GetUserID returns the authenticated user. Only meaningful on routes behind
// middleware.Auth.
func GetUserID(c *gin.Context) uuid.UUID {
	id, _ := c.Get(CtxUserID)
	userID, _ := id.(uuid.UUID)
	return userID
}

// ParamUUID reads a path param as UUID; on failure it writes a 400 and
// returns ok=false so the controller can just return.
func ParamUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		RespondError(c, ErrBadRequest("invalid "+name))
		return uuid.Nil, false
	}
	return id, true
}
