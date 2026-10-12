package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/model"
)

// Repository contracts the services depend on. The concrete Postgres
// implementations live in internal/repository; tests use in-memory fakes.

type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	FindByEmail(ctx context.Context, email string) (model.User, error)
}

type TeamRepository interface {
	Create(ctx context.Context, t *model.Team) error
	FindByID(ctx context.Context, id uuid.UUID) (model.Team, error)
	ListByMember(ctx context.Context, userID uuid.UUID) ([]model.Team, error)
	Update(ctx context.Context, t *model.Team) error
	Delete(ctx context.Context, id uuid.UUID) error
	AddMember(ctx context.Context, m model.TeamMember) error
	RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error
	FindMember(ctx context.Context, teamID, userID uuid.UUID) (model.TeamMember, error)
	ListMembers(ctx context.Context, teamID uuid.UUID) ([]model.TeamMember, error)
}

type TaskRepository interface {
	Create(ctx context.Context, t *model.Task) error
	FindByID(ctx context.Context, id uuid.UUID) (model.Task, error)
	FindByIDForUpdate(ctx context.Context, id uuid.UUID) (model.Task, error)
	List(ctx context.Context, f model.TaskFilter) ([]model.Task, error)
	Count(ctx context.Context, f model.TaskFilter) (int64, error)
	Update(ctx context.Context, t *model.Task) error
	UpdateAssignee(ctx context.Context, t *model.Task) error
	Delete(ctx context.Context, id uuid.UUID) error
	CreateLog(ctx context.Context, l *model.TaskLog) error
}

type IdempotencyRepository interface {
	Claim(ctx context.Context, k model.IdempotencyKey) (bool, error)
	Find(ctx context.Context, userID, key uuid.UUID) (model.IdempotencyKey, error)
	SaveResponse(ctx context.Context, userID, key uuid.UUID, body []byte) error
}
