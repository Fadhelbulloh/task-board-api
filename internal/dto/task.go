package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/model"
)

type TaskRequest struct {
	Title       string     `json:"title" binding:"required,max=200"`
	Description string     `json:"description" binding:"max=5000"`
	Status      string     `json:"status" binding:"omitempty,oneof=todo in_progress done"`
	TeamID      *uuid.UUID `json:"team_id"`
}

// ListTaskQuery binds GET /tasks query params. Defaults are applied in the
// controller before validation so "no limit" means 20, not 0.
type ListTaskQuery struct {
	Status string `form:"status" binding:"omitempty,oneof=todo in_progress done"`
	Search string `form:"search" binding:"max=200"`
	Page   int    `form:"page" binding:"min=1"`
	Limit  int    `form:"limit" binding:"min=1,max=100"`
}

type AssignTaskRequest struct {
	AssigneeID uuid.UUID `json:"assignee_id" binding:"required"`
}

type TaskResponse struct {
	ID          uuid.UUID  `json:"id"`
	OwnerID     uuid.UUID  `json:"owner_id"`
	AssigneeID  *uuid.UUID `json:"assignee_id"`
	TeamID      *uuid.UUID `json:"team_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Pagination struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
}

type TaskListResponse struct {
	Data       []TaskResponse `json:"data"`
	Pagination Pagination     `json:"pagination"`
}

func NewTaskResponse(t model.Task) TaskResponse {
	return TaskResponse{
		ID:          t.ID,
		OwnerID:     t.OwnerID,
		AssigneeID:  t.AssigneeID,
		TeamID:      t.TeamID,
		Title:       t.Title,
		Description: t.Description,
		Status:      string(t.Status),
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
