package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Fadhelbulloh/task-board-api/internal/model"
)

type TaskRepository struct {
	pool *pgxpool.Pool
}

func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{pool: pool}
}

func (r *TaskRepository) Create(ctx context.Context, t *model.Task) error {
	err := conn(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO tasks (owner_id, team_id, title, description, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		t.OwnerID, t.TeamID, t.Title, t.Description, t.Status,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	return mapErr(err)
}

func (r *TaskRepository) FindByID(ctx context.Context, id uuid.UUID) (model.Task, error) {
	row := conn(ctx, r.pool).QueryRow(ctx, `
		SELECT id, owner_id, assignee_id, team_id, title, description, status, created_at, updated_at
		FROM tasks
		WHERE id = $1`,
		id,
	)
	t, err := scanTask(row)
	return t, mapErr(err)
}

// FindByIDForUpdate locks the row until the surrounding transaction ends so
// two concurrent assigns on the same task serialise. Must be called inside
// Transactor.WithinTx.
func (r *TaskRepository) FindByIDForUpdate(ctx context.Context, id uuid.UUID) (model.Task, error) {
	row := conn(ctx, r.pool).QueryRow(ctx, `
		SELECT id, owner_id, assignee_id, team_id, title, description, status, created_at, updated_at
		FROM tasks
		WHERE id = $1
		FOR UPDATE`,
		id,
	)
	t, err := scanTask(row)
	return t, mapErr(err)
}

// List returns one page of tasks visible to f.UserID, newest first.
func (r *TaskRepository) List(ctx context.Context, f model.TaskFilter) ([]model.Task, error) {
	where, args := taskFilterWhere(f)
	args = append(args, f.Limit, f.Offset)

	query := fmt.Sprintf(`
		SELECT id, owner_id, assignee_id, team_id, title, description, status, created_at, updated_at
		FROM tasks
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`,
		where, len(args)-1, len(args),
	)

	rows, err := conn(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []model.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *TaskRepository) Count(ctx context.Context, f model.TaskFilter) (int64, error) {
	where, args := taskFilterWhere(f)
	var total int64
	err := conn(ctx, r.pool).QueryRow(ctx, `SELECT COUNT(*) FROM tasks WHERE `+where, args...).Scan(&total)
	return total, err
}

func (r *TaskRepository) Update(ctx context.Context, t *model.Task) error {
	err := conn(ctx, r.pool).QueryRow(ctx, `
		UPDATE tasks
		SET title = $2, description = $3, status = $4, team_id = $5, updated_at = now()
		WHERE id = $1
		RETURNING updated_at`,
		t.ID, t.Title, t.Description, t.Status, t.TeamID,
	).Scan(&t.UpdatedAt)
	return mapErr(err)
}

func (r *TaskRepository) UpdateAssignee(ctx context.Context, t *model.Task) error {
	err := conn(ctx, r.pool).QueryRow(ctx, `
		UPDATE tasks
		SET assignee_id = $2, updated_at = now()
		WHERE id = $1
		RETURNING updated_at`,
		t.ID, t.AssigneeID,
	).Scan(&t.UpdatedAt)
	return mapErr(err)
}

func (r *TaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := conn(ctx, r.pool).Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TaskRepository) CreateLog(ctx context.Context, l *model.TaskLog) error {
	err := conn(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO task_logs (task_id, actor_id, action, old_value, new_value)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		l.TaskID, l.ActorID, l.Action, l.OldValue, l.NewValue,
	).Scan(&l.ID, &l.CreatedAt)
	return mapErr(err)
}

// taskFilterWhere builds the WHERE clause shared by List and Count. Only
// filters that are set end up in the SQL, so the planner sees a simple
// query and can use the indexes from migration 000003.
func taskFilterWhere(f model.TaskFilter) (string, []any) {
	conds := []string{"(owner_id = $1 OR assignee_id = $1)"}
	args := []any{f.UserID}

	if f.Status != nil {
		args = append(args, *f.Status)
		conds = append(conds, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.Search != "" {
		args = append(args, escapeLike(strings.ToLower(f.Search))+"%")
		conds = append(conds, fmt.Sprintf("lower(title) LIKE $%d", len(args)))
	}
	return strings.Join(conds, " AND "), args
}

// escapeLike stops user input like "50%" from acting as a wildcard.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func scanTask(row pgx.Row) (model.Task, error) {
	var t model.Task
	err := row.Scan(&t.ID, &t.OwnerID, &t.AssigneeID, &t.TeamID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}
