// Package mock provides in-memory implementations of the repository
// interfaces in internal/service, so unit tests run without Postgres.
//
// Two Postgres behaviours the services rely on are reproduced:
//   - Rollback: every write inside Transactor.WithinTx records an undo step;
//     if fn returns an error the steps run in reverse.
//   - Row lock on insert: Claim on a key that another open transaction has
//     just inserted blocks until that transaction ends, exactly like a
//     second INSERT on the same primary key does in Postgres.
//
// Reads are not isolated between transactions. Tests that need real
// isolation levels belong in an integration suite against Postgres.
package mock

import (
	"context"
	"slices"
	"sync"

	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/model"
)

type memberKey struct{ teamID, userID uuid.UUID }
type idemKey struct{ userID, key uuid.UUID }

type DB struct {
	mu       sync.Mutex
	users    map[uuid.UUID]model.User
	teams    map[uuid.UUID]model.Team
	members  map[memberKey]model.TeamMember
	tasks    map[uuid.UUID]model.Task
	taskLogs []model.TaskLog
	idemKeys map[idemKey]model.IdempotencyKey
	// idemLocks holds keys inserted by a transaction that hasn't finished.
	// The channel is closed when it commits or rolls back.
	idemLocks map[idemKey]*tx
}

func NewDB() *DB {
	return &DB{
		users:     map[uuid.UUID]model.User{},
		teams:     map[uuid.UUID]model.Team{},
		members:   map[memberKey]model.TeamMember{},
		tasks:     map[uuid.UUID]model.Task{},
		idemKeys:  map[idemKey]model.IdempotencyKey{},
		idemLocks: map[idemKey]*tx{},
	}
}

type tx struct {
	undo []func() // run with db.mu held
	done chan struct{}
}

type txCtxKey struct{}

func txFrom(ctx context.Context) *tx {
	t, _ := ctx.Value(txCtxKey{}).(*tx)
	return t
}

// onRollback registers how to revert a write. No-op outside a transaction.
// Caller must hold db.mu.
func (db *DB) onRollback(ctx context.Context, undo func()) {
	if t := txFrom(ctx); t != nil {
		t.undo = append(t.undo, undo)
	}
}

// Assertion helpers.

func (db *DB) TaskCount() int {
	db.mu.Lock()
	defer db.mu.Unlock()
	return len(db.tasks)
}

func (db *DB) Task(id uuid.UUID) model.Task {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.tasks[id]
}

func (db *DB) TaskLogs() []model.TaskLog {
	db.mu.Lock()
	defer db.mu.Unlock()
	return slices.Clone(db.taskLogs)
}

func (db *DB) IdempotencyKeyCount() int {
	db.mu.Lock()
	defer db.mu.Unlock()
	return len(db.idemKeys)
}

// Seed helpers.

func (db *DB) AddTeamMember(m model.TeamMember) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.members[memberKey{m.TeamID, m.UserID}] = m
}

func (db *DB) PutTask(t model.Task) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.tasks[t.ID] = t
}
