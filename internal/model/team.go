package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	TeamRoleOwner  = "owner"
	TeamRoleMember = "member"
)

type Team struct {
	ID        uuid.UUID
	Name      string
	OwnerID   uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TeamMember struct {
	TeamID   uuid.UUID
	UserID   uuid.UUID
	Role     string
	JoinedAt time.Time

	// Joined from users; only filled by TeamRepository.ListMembers.
	UserEmail string
	UserName  string
}
