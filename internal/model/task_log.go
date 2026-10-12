package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const TaskActionAssign = "assign"

type TaskLog struct {
	ID        int64
	TaskID    uuid.UUID
	ActorID   uuid.UUID
	Action    string
	OldValue  json.RawMessage
	NewValue  json.RawMessage
	CreatedAt time.Time
}
