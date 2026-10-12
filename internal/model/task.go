package model

import (
	"time"

	"github.com/google/uuid"
)

// TaskStatus mirrors the task_status enum in migrations/000003.
type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
)

type Task struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	AssigneeID  *uuid.UUID
	TeamID      *uuid.UUID
	Title       string
	Description string
	Status      TaskStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsVisibleTo is the single rule for "can this user see the task":
// owner or current assignee.
func (t Task) IsVisibleTo(userID uuid.UUID) bool {
	return t.OwnerID == userID || (t.AssigneeID != nil && *t.AssigneeID == userID)
}

// TaskFilter is what TaskRepository.List/Count accept.
type TaskFilter struct {
	UserID uuid.UUID
	Status *TaskStatus
	Search string // title prefix, case-insensitive
	Limit  int
	Offset int
}
