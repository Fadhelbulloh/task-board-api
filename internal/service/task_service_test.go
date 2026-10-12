package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/dto"
	"github.com/Fadhelbulloh/task-board-api/internal/helper"
	"github.com/Fadhelbulloh/task-board-api/internal/integration"
	"github.com/Fadhelbulloh/task-board-api/internal/mock"
	"github.com/Fadhelbulloh/task-board-api/internal/model"
)

type stubNotifier struct {
	err   error
	calls int
}

func (n *stubNotifier) NotifyTaskAssigned(context.Context, integration.TaskAssignedEvent) error {
	n.calls++
	return n.err
}

type assignFixture struct {
	db       *mock.DB
	tasks    *mock.TaskRepository
	notifier *stubNotifier
	svc      *TaskService

	owner, member, outsider uuid.UUID
	task                    model.Task
}

// newAssignFixture: owner and member share a team, outsider does not.
// The task belongs to owner and is attached to the team.
func newAssignFixture() *assignFixture {
	db := mock.NewDB()
	f := &assignFixture{
		db:       db,
		tasks:    &mock.TaskRepository{DB: db},
		notifier: &stubNotifier{},
		owner:    uuid.New(),
		member:   uuid.New(),
		outsider: uuid.New(),
	}
	teamID := uuid.New()
	db.AddTeamMember(model.TeamMember{TeamID: teamID, UserID: f.owner, Role: model.TeamRoleOwner})
	db.AddTeamMember(model.TeamMember{TeamID: teamID, UserID: f.member, Role: model.TeamRoleMember})

	f.task = model.Task{ID: uuid.New(), OwnerID: f.owner, TeamID: &teamID, Title: "Deploy", Status: model.TaskStatusTodo}
	db.PutTask(f.task)

	f.svc = NewTaskService(mock.Transactor{DB: db}, f.tasks, mock.TeamRepository{DB: db}, &mock.IdempotencyRepository{DB: db}, time.Hour, f.notifier)
	return f
}

func (f *assignFixture) assign(actor, assignee uuid.UUID) (dto.TaskResponse, error) {
	return f.svc.Assign(context.Background(), actor, f.task.ID, dto.AssignTaskRequest{AssigneeID: assignee})
}

func requireAppError(t *testing.T, err error, status int) {
	t.Helper()
	var appErr *helper.AppError
	if !errors.As(err, &appErr) || appErr.Status != status {
		t.Fatalf("got error %v, want status %d", err, status)
	}
}

func TestAssign_Success(t *testing.T) {
	f := newAssignFixture()

	res, err := f.assign(f.owner, f.member)
	if err != nil {
		t.Fatal(err)
	}
	if res.AssigneeID == nil || *res.AssigneeID != f.member {
		t.Fatalf("assignee = %v, want %v", res.AssigneeID, f.member)
	}
	logs := f.db.TaskLogs()
	if len(logs) != 1 || logs[0].Action != model.TaskActionAssign || logs[0].ActorID != f.owner {
		t.Fatalf("task_logs = %+v", logs)
	}
	if f.notifier.calls != 1 {
		t.Fatalf("notifier called %d times, want 1", f.notifier.calls)
	}
}

func TestAssign_RollbackWhenTaskLogFails(t *testing.T) {
	f := newAssignFixture()
	f.tasks.CreateLogErr = errors.New("insert task_logs: connection reset")

	_, err := f.assign(f.owner, f.member)
	requireAppError(t, err, http.StatusInternalServerError)

	if f.db.Task(f.task.ID).AssigneeID != nil {
		t.Fatal("assignee was saved even though task_logs insert failed")
	}
	if f.notifier.calls != 0 {
		t.Fatal("notification sent for a rolled back assignment")
	}
}

func TestAssign_RollbackWhenNotificationFails(t *testing.T) {
	f := newAssignFixture()
	f.notifier.err = errors.New("smtp timeout")

	_, err := f.assign(f.owner, f.member)
	requireAppError(t, err, http.StatusBadGateway)

	if f.db.Task(f.task.ID).AssigneeID != nil {
		t.Fatal("assignee was saved even though notification failed")
	}
	if len(f.db.TaskLogs()) != 0 {
		t.Fatal("task_logs row was kept even though notification failed")
	}
}

func TestAssign_AssigneeMustBeTeamMember(t *testing.T) {
	f := newAssignFixture()
	_, err := f.assign(f.owner, f.outsider)
	requireAppError(t, err, http.StatusUnprocessableEntity)
}

func TestAssign_OutsiderCannotSeeTask(t *testing.T) {
	f := newAssignFixture()
	_, err := f.assign(f.outsider, f.member)
	requireAppError(t, err, http.StatusNotFound)
}

func TestAssign_TaskWithoutTeam(t *testing.T) {
	f := newAssignFixture()
	f.task.TeamID = nil
	f.db.PutTask(f.task)

	_, err := f.assign(f.owner, f.member)
	requireAppError(t, err, http.StatusUnprocessableEntity)
}
