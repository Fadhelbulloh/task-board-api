package service

import (
	"context"
	"errors"
	"strings"

	"github.com/Fadhelbulloh/task-board-api/internal/dto"
	"github.com/Fadhelbulloh/task-board-api/internal/helper"
	"github.com/Fadhelbulloh/task-board-api/internal/model"
	"github.com/Fadhelbulloh/task-board-api/internal/repository"
)

type AuthService struct {
	users UserRepository
	jwt   *helper.JWT
}

func NewAuthService(users UserRepository, jwt *helper.JWT) *AuthService {
	return &AuthService{users: users, jwt: jwt}
}

func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (dto.UserResponse, error) {
	hash, err := helper.HashPassword(req.Password)
	if err != nil {
		return dto.UserResponse{}, helper.ErrInternal(err)
	}

	user := model.User{
		Email:        normalizeEmail(req.Email),
		Name:         strings.TrimSpace(req.Name),
		PasswordHash: hash,
	}
	if err := s.users.Create(ctx, &user); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return dto.UserResponse{}, helper.ErrConflict("EMAIL_ALREADY_REGISTERED", "email is already registered")
		}
		return dto.UserResponse{}, helper.ErrInternal(err)
	}
	return dto.NewUserResponse(user), nil
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (dto.LoginResponse, error) {
	// Same error for unknown email and wrong password so the endpoint can't
	// be used to discover registered emails.
	invalid := helper.ErrUnauthorized("invalid email or password")

	user, err := s.users.FindByEmail(ctx, normalizeEmail(req.Email))
	if errors.Is(err, repository.ErrNotFound) {
		return dto.LoginResponse{}, invalid
	}
	if err != nil {
		return dto.LoginResponse{}, helper.ErrInternal(err)
	}
	if !helper.CheckPassword(user.PasswordHash, req.Password) {
		return dto.LoginResponse{}, invalid
	}

	token, expiresAt, err := s.jwt.Generate(user.ID)
	if err != nil {
		return dto.LoginResponse{}, helper.ErrInternal(err)
	}
	return dto.LoginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		User:        dto.NewUserResponse(user),
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
