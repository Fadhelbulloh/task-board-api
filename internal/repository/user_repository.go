package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Fadhelbulloh/task-board-api/internal/model"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	err := conn(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`,
		u.Email, u.Name, u.PasswordHash,
	).Scan(&u.ID, &u.CreatedAt)
	return mapErr(err)
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (model.User, error) {
	var u model.User
	err := conn(ctx, r.pool).QueryRow(ctx, `
		SELECT id, email, name, password_hash, created_at
		FROM users
		WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt)
	return u, mapErr(err)
}
