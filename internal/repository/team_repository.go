package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Fadhelbulloh/task-board-api/internal/model"
)

type TeamRepository struct {
	pool *pgxpool.Pool
}

func NewTeamRepository(pool *pgxpool.Pool) *TeamRepository {
	return &TeamRepository{pool: pool}
}

func (r *TeamRepository) Create(ctx context.Context, t *model.Team) error {
	err := conn(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO teams (name, owner_id)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at`,
		t.Name, t.OwnerID,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	return mapErr(err)
}

func (r *TeamRepository) FindByID(ctx context.Context, id uuid.UUID) (model.Team, error) {
	var t model.Team
	err := conn(ctx, r.pool).QueryRow(ctx, `
		SELECT id, name, owner_id, created_at, updated_at
		FROM teams
		WHERE id = $1`,
		id,
	).Scan(&t.ID, &t.Name, &t.OwnerID, &t.CreatedAt, &t.UpdatedAt)
	return t, mapErr(err)
}

func (r *TeamRepository) ListByMember(ctx context.Context, userID uuid.UUID) ([]model.Team, error) {
	rows, err := conn(ctx, r.pool).Query(ctx, `
		SELECT t.id, t.name, t.owner_id, t.created_at, t.updated_at
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = $1
		ORDER BY t.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	teams := []model.Team{}
	for rows.Next() {
		var t model.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.OwnerID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

func (r *TeamRepository) Update(ctx context.Context, t *model.Team) error {
	err := conn(ctx, r.pool).QueryRow(ctx, `
		UPDATE teams
		SET name = $2, updated_at = now()
		WHERE id = $1
		RETURNING owner_id, created_at, updated_at`,
		t.ID, t.Name,
	).Scan(&t.OwnerID, &t.CreatedAt, &t.UpdatedAt)
	return mapErr(err)
}

func (r *TeamRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := conn(ctx, r.pool).Exec(ctx, `DELETE FROM teams WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TeamRepository) AddMember(ctx context.Context, m model.TeamMember) error {
	_, err := conn(ctx, r.pool).Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES ($1, $2, $3)`,
		m.TeamID, m.UserID, m.Role,
	)
	return mapErr(err)
}

func (r *TeamRepository) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	tag, err := conn(ctx, r.pool).Exec(ctx, `
		DELETE FROM team_members
		WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TeamRepository) FindMember(ctx context.Context, teamID, userID uuid.UUID) (model.TeamMember, error) {
	m := model.TeamMember{TeamID: teamID, UserID: userID}
	err := conn(ctx, r.pool).QueryRow(ctx, `
		SELECT role, joined_at
		FROM team_members
		WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	).Scan(&m.Role, &m.JoinedAt)
	return m, mapErr(err)
}

func (r *TeamRepository) ListMembers(ctx context.Context, teamID uuid.UUID) ([]model.TeamMember, error) {
	rows, err := conn(ctx, r.pool).Query(ctx, `
		SELECT tm.user_id, tm.role, tm.joined_at, u.email, u.name
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.team_id = $1
		ORDER BY tm.joined_at`,
		teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []model.TeamMember{}
	for rows.Next() {
		m := model.TeamMember{TeamID: teamID}
		if err := rows.Scan(&m.UserID, &m.Role, &m.JoinedAt, &m.UserEmail, &m.UserName); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}
