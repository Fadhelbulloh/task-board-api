package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/model"
)

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Name     string `json:"name" binding:"required,max=100"`
	Password string `json:"password" binding:"required,min=8,max=72"` // bcrypt ignores bytes past 72
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresAt   time.Time    `json:"expires_at"`
	User        UserResponse `json:"user"`
}

func NewUserResponse(u model.User) UserResponse {
	return UserResponse{ID: u.ID, Email: u.Email, Name: u.Name, CreatedAt: u.CreatedAt}
}
