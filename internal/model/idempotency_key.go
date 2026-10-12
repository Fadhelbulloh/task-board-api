package model

import (
	"time"

	"github.com/google/uuid"
)

type IdempotencyKey struct {
	UserID       uuid.UUID
	Key          uuid.UUID
	RequestHash  string
	ResponseBody []byte
	CreatedAt    time.Time
	ExpiresAt    time.Time
}
