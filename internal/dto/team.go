package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/model"
)

type TeamRequest struct {
	Name string `json:"name" binding:"required,max=100"`
}

type AddMemberRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type TeamResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	OwnerID   uuid.UUID `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TeamMemberResponse struct {
	UserID   uuid.UUID `json:"user_id"`
	Email    string    `json:"email"`
	Name     string    `json:"name"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

func NewTeamResponse(t model.Team) TeamResponse {
	return TeamResponse{ID: t.ID, Name: t.Name, OwnerID: t.OwnerID, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

func NewTeamMemberResponse(m model.TeamMember) TeamMemberResponse {
	return TeamMemberResponse{UserID: m.UserID, Email: m.UserEmail, Name: m.UserName, Role: m.Role, JoinedAt: m.JoinedAt}
}
