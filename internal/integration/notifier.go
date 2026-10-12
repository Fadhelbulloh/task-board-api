// Package integration holds clients for third-party services. The spec lets
// the assignment notification be a mock, so LogNotifier just writes a log
// line. A real email/push client would implement the same interface.
package integration

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

type TaskAssignedEvent struct {
	TaskID     uuid.UUID
	TaskTitle  string
	AssigneeID uuid.UUID
	AssignedBy uuid.UUID
}

type Notifier interface {
	NotifyTaskAssigned(ctx context.Context, event TaskAssignedEvent) error
}

type LogNotifier struct{}

func NewLogNotifier() *LogNotifier { return &LogNotifier{} }

func (LogNotifier) NotifyTaskAssigned(ctx context.Context, e TaskAssignedEvent) error {
	slog.InfoContext(ctx, "notification sent: task assigned",
		"task_id", e.TaskID,
		"task_title", e.TaskTitle,
		"assignee_id", e.AssigneeID,
		"assigned_by", e.AssignedBy,
	)
	return nil
}
