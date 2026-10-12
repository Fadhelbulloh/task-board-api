package mock

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/model"
	"github.com/Fadhelbulloh/task-board-api/internal/repository"
)

// Transactor

type Transactor struct{ DB *DB }

func (t Transactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	current := &tx{done: make(chan struct{})}
	err := fn(context.WithValue(ctx, txCtxKey{}, current))

	t.DB.mu.Lock()
	if err != nil {
		for i := len(current.undo) - 1; i >= 0; i-- {
			current.undo[i]()
		}
	}
	for k, owner := range t.DB.idemLocks {
		if owner == current {
			delete(t.DB.idemLocks, k)
		}
	}
	t.DB.mu.Unlock()
	close(current.done) // wake up Claims waiting on our keys
	return err
}

// UserRepository

type UserRepository struct{ DB *DB }

func (r UserRepository) Create(ctx context.Context, u *model.User) error {
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	for _, existing := range r.DB.users {
		if existing.Email == u.Email {
			return repository.ErrDuplicate
		}
	}
	u.ID, u.CreatedAt = uuid.New(), time.Now()
	r.DB.users[u.ID] = *u
	id := u.ID
	r.DB.onRollback(ctx, func() { delete(r.DB.users, id) })
	return nil
}

func (r UserRepository) FindByEmail(_ context.Context, email string) (model.User, error) {
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	for _, u := range r.DB.users {
		if u.Email == email {
			return u, nil
		}
	}
	return model.User{}, repository.ErrNotFound
}

// TeamRepository. Only what the task and auth tests touch does real work;
// the rest exists to satisfy the interface.

type TeamRepository struct{ DB *DB }

func (r TeamRepository) FindMember(_ context.Context, teamID, userID uuid.UUID) (model.TeamMember, error) {
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	m, ok := r.DB.members[memberKey{teamID, userID}]
	if !ok {
		return model.TeamMember{}, repository.ErrNotFound
	}
	return m, nil
}

func (r TeamRepository) AddMember(ctx context.Context, m model.TeamMember) error {
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	k := memberKey{m.TeamID, m.UserID}
	if _, ok := r.DB.members[k]; ok {
		return repository.ErrDuplicate
	}
	r.DB.members[k] = m
	r.DB.onRollback(ctx, func() { delete(r.DB.members, k) })
	return nil
}

func (r TeamRepository) Create(ctx context.Context, t *model.Team) error {
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	t.ID, t.CreatedAt, t.UpdatedAt = uuid.New(), time.Now(), time.Now()
	r.DB.teams[t.ID] = *t
	id := t.ID
	r.DB.onRollback(ctx, func() { delete(r.DB.teams, id) })
	return nil
}

func (r TeamRepository) FindByID(_ context.Context, id uuid.UUID) (model.Team, error) {
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	t, ok := r.DB.teams[id]
	if !ok {
		return model.Team{}, repository.ErrNotFound
	}
	return t, nil
}

func (r TeamRepository) ListByMember(context.Context, uuid.UUID) ([]model.Team, error) {
	return []model.Team{}, nil
}
func (r TeamRepository) Update(context.Context, *model.Team) error                { return nil }
func (r TeamRepository) Delete(context.Context, uuid.UUID) error                  { return nil }
func (r TeamRepository) RemoveMember(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r TeamRepository) ListMembers(context.Context, uuid.UUID) ([]model.TeamMember, error) {
	return []model.TeamMember{}, nil
}

// TaskRepository

type TaskRepository struct {
	DB *DB
	// CreateDelay keeps a request inside its transaction for a while, so
	// concurrent requests really overlap.
	CreateDelay time.Duration
	// CreateLogErr makes CreateLog fail, to exercise rollback.
	CreateLogErr error
}

func (r *TaskRepository) Create(ctx context.Context, t *model.Task) error {
	time.Sleep(r.CreateDelay)
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	t.ID, t.CreatedAt, t.UpdatedAt = uuid.New(), time.Now(), time.Now()
	r.DB.tasks[t.ID] = *t
	id := t.ID
	r.DB.onRollback(ctx, func() { delete(r.DB.tasks, id) })
	return nil
}

func (r *TaskRepository) FindByID(_ context.Context, id uuid.UUID) (model.Task, error) {
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	t, ok := r.DB.tasks[id]
	if !ok {
		return model.Task{}, repository.ErrNotFound
	}
	return t, nil
}

func (r *TaskRepository) FindByIDForUpdate(ctx context.Context, id uuid.UUID) (model.Task, error) {
	return r.FindByID(ctx, id)
}

func (r *TaskRepository) List(_ context.Context, f model.TaskFilter) ([]model.Task, error) {
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	var out []model.Task
	for _, t := range r.DB.tasks {
		if !t.IsVisibleTo(f.UserID) {
			continue
		}
		if f.Status != nil && t.Status != *f.Status {
			continue
		}
		if f.Search != "" && !strings.HasPrefix(strings.ToLower(t.Title), strings.ToLower(f.Search)) {
			continue
		}
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b model.Task) int { return b.CreatedAt.Compare(a.CreatedAt) })
	if f.Offset >= len(out) {
		return []model.Task{}, nil
	}
	return out[f.Offset:min(f.Offset+f.Limit, len(out))], nil
}

func (r *TaskRepository) Count(ctx context.Context, f model.TaskFilter) (int64, error) {
	f.Offset, f.Limit = 0, 1<<30
	all, err := r.List(ctx, f)
	return int64(len(all)), err
}

func (r *TaskRepository) Update(ctx context.Context, t *model.Task) error {
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	before, ok := r.DB.tasks[t.ID]
	if !ok {
		return repository.ErrNotFound
	}
	t.UpdatedAt = time.Now()
	r.DB.tasks[t.ID] = *t
	r.DB.onRollback(ctx, func() { r.DB.tasks[before.ID] = before })
	return nil
}

func (r *TaskRepository) UpdateAssignee(ctx context.Context, t *model.Task) error {
	return r.Update(ctx, t)
}

func (r *TaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	before, ok := r.DB.tasks[id]
	if !ok {
		return repository.ErrNotFound
	}
	delete(r.DB.tasks, id)
	r.DB.onRollback(ctx, func() { r.DB.tasks[id] = before })
	return nil
}

func (r *TaskRepository) CreateLog(ctx context.Context, l *model.TaskLog) error {
	if r.CreateLogErr != nil {
		return r.CreateLogErr
	}
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	l.ID, l.CreatedAt = int64(len(r.DB.taskLogs)+1), time.Now()
	r.DB.taskLogs = append(r.DB.taskLogs, *l)
	r.DB.onRollback(ctx, func() { r.DB.taskLogs = r.DB.taskLogs[:len(r.DB.taskLogs)-1] })
	return nil
}

// IdempotencyRepository

type IdempotencyRepository struct {
	DB *DB
	// SaveResponseErr makes SaveResponse fail, to check nothing is left
	// half-written.
	SaveResponseErr error
}

// Claim behaves like INSERT ... ON CONFLICT DO UPDATE ... WHERE expired:
// if another open transaction inserted the key, wait for it to finish;
// then the key is either taken (false) or free to insert (true).
func (r *IdempotencyRepository) Claim(ctx context.Context, k model.IdempotencyKey) (bool, error) {
	id := idemKey{k.UserID, k.Key}
	current := txFrom(ctx)
	for {
		r.DB.mu.Lock()
		owner, locked := r.DB.idemLocks[id]
		if !locked || owner == current {
			break // keep holding mu
		}
		r.DB.mu.Unlock()
		select {
		case <-owner.done:
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	defer r.DB.mu.Unlock()

	before, exists := r.DB.idemKeys[id]
	if exists && before.ExpiresAt.After(time.Now()) {
		return false, nil
	}
	k.CreatedAt = time.Now()
	r.DB.idemKeys[id] = k
	if current != nil {
		r.DB.idemLocks[id] = current
	}
	r.DB.onRollback(ctx, func() {
		if exists {
			r.DB.idemKeys[id] = before
		} else {
			delete(r.DB.idemKeys, id)
		}
	})
	return true, nil
}

func (r *IdempotencyRepository) Find(_ context.Context, userID, key uuid.UUID) (model.IdempotencyKey, error) {
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	k, ok := r.DB.idemKeys[idemKey{userID, key}]
	if !ok {
		return model.IdempotencyKey{}, repository.ErrNotFound
	}
	return k, nil
}

func (r *IdempotencyRepository) SaveResponse(ctx context.Context, userID, key uuid.UUID, body []byte) error {
	if r.SaveResponseErr != nil {
		return r.SaveResponseErr
	}
	r.DB.mu.Lock()
	defer r.DB.mu.Unlock()
	id := idemKey{userID, key}
	before := r.DB.idemKeys[id]
	after := before
	after.ResponseBody = slices.Clone(body)
	r.DB.idemKeys[id] = after
	r.DB.onRollback(ctx, func() { r.DB.idemKeys[id] = before })
	return nil
}
