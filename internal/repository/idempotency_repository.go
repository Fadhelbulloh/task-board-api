package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Fadhelbulloh/task-board-api/internal/model"
)

type IdempotencyRepository struct {
	pool *pgxpool.Pool
}

func NewIdempotencyRepository(pool *pgxpool.Pool) *IdempotencyRepository {
	return &IdempotencyRepository{pool: pool}
}

// Claim inserts the key and reports whether this caller got it. Call it
// inside a transaction: a second INSERT on the same primary key waits until
// the first transaction commits or rolls back, then either conflicts
// (returns false) or goes through (returns true). An expired row is
// overwritten as if it didn't exist.
func (r *IdempotencyRepository) Claim(ctx context.Context, k model.IdempotencyKey) (bool, error) {
	tag, err := conn(ctx, r.pool).Exec(ctx, `
		INSERT INTO idempotency_keys (user_id, key, request_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, key) DO UPDATE
		SET request_hash  = EXCLUDED.request_hash,
		    response_body = NULL,
		    created_at    = now(),
		    expires_at    = EXCLUDED.expires_at
		WHERE idempotency_keys.expires_at < now()`,
		k.UserID, k.Key, k.RequestHash, k.ExpiresAt,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *IdempotencyRepository) Find(ctx context.Context, userID, key uuid.UUID) (model.IdempotencyKey, error) {
	k := model.IdempotencyKey{UserID: userID, Key: key}
	err := conn(ctx, r.pool).QueryRow(ctx, `
		SELECT request_hash, response_body, created_at, expires_at
		FROM idempotency_keys
		WHERE user_id = $1 AND key = $2`,
		userID, key,
	).Scan(&k.RequestHash, &k.ResponseBody, &k.CreatedAt, &k.ExpiresAt)
	return k, mapErr(err)
}

func (r *IdempotencyRepository) SaveResponse(ctx context.Context, userID, key uuid.UUID, body []byte) error {
	_, err := conn(ctx, r.pool).Exec(ctx, `
		UPDATE idempotency_keys
		SET response_body = $3
		WHERE user_id = $1 AND key = $2`,
		userID, key, body,
	)
	return err
}
