package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/dto"
	"github.com/Fadhelbulloh/task-board-api/internal/helper"
	"github.com/Fadhelbulloh/task-board-api/internal/integration"
	"github.com/Fadhelbulloh/task-board-api/internal/model"
	"github.com/Fadhelbulloh/task-board-api/internal/repository"
)

type TaskService struct {
	tx             Transactor
	tasks          TaskRepository
	teams          TeamRepository
	idempotency    IdempotencyRepository
	idempotencyTTL time.Duration
	notifier       integration.Notifier
}

func NewTaskService(
	tx Transactor,
	tasks TaskRepository,
	teams TeamRepository,
	idempotency IdempotencyRepository,
	idempotencyTTL time.Duration,
	notifier integration.Notifier,
) *TaskService {
	return &TaskService{
		tx:             tx,
		tasks:          tasks,
		teams:          teams,
		idempotency:    idempotency,
		idempotencyTTL: idempotencyTTL,
		notifier:       notifier,
	}
}

// Create makes at most one task per Idempotency-Key (spec "Idempotency").
// replayed is true when the response comes from an earlier request.
//
// Everything runs in one transaction:
//  1. Claim the key. If another request is holding it, Postgres makes this
//     INSERT wait until that transaction commits or rolls back.
//  2. Key already used: return the stored response, or 422 if the payload
//     is different.
//  3. Key is ours: insert the task and store its response next to the key.
//
// The key, the task and the stored response commit or roll back together.
// So a task can't exist without its key (which would allow a duplicate on
// retry), and a key can't exist without a response (which would block
// retries after a crash).
func (s *TaskService) Create(ctx context.Context, userID, idempotencyKey uuid.UUID, req dto.TaskRequest) (res dto.TaskResponse, replayed bool, err error) {
	requestHash, err := hashRequest(req)
	if err != nil {
		return dto.TaskResponse{}, false, helper.ErrInternal(err)
	}

	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		claimed, err := s.idempotency.Claim(ctx, model.IdempotencyKey{
			UserID:      userID,
			Key:         idempotencyKey,
			RequestHash: requestHash,
			ExpiresAt:   time.Now().Add(s.idempotencyTTL),
		})
		if err != nil {
			return helper.ErrInternal(err)
		}
		if !claimed {
			res, err = s.storedResponse(ctx, userID, idempotencyKey, requestHash)
			replayed = err == nil
			return err
		}

		res, err = s.insertTask(ctx, userID, req)
		if err != nil {
			return err
		}
		body, err := json.Marshal(res)
		if err != nil {
			return helper.ErrInternal(err)
		}
		if err := s.idempotency.SaveResponse(ctx, userID, idempotencyKey, body); err != nil {
			return helper.ErrInternal(err)
		}
		return nil
	})
	if err != nil {
		return dto.TaskResponse{}, false, helper.AsAppError(err)
	}
	return res, replayed, nil
}

func (s *TaskService) insertTask(ctx context.Context, userID uuid.UUID, req dto.TaskRequest) (dto.TaskResponse, error) {
	if err := s.ensureTeamMember(ctx, req.TeamID, userID); err != nil {
		return dto.TaskResponse{}, err
	}
	task := model.Task{
		OwnerID:     userID,
		TeamID:      req.TeamID,
		Title:       strings.TrimSpace(req.Title),
		Description: req.Description,
		Status:      statusOrDefault(req.Status),
	}
	if err := s.tasks.Create(ctx, &task); err != nil {
		return dto.TaskResponse{}, helper.ErrInternal(err)
	}
	return dto.NewTaskResponse(task), nil
}

// storedResponse loads the response saved by the request that first used
// this key. The row is always committed by the time we get here, because
// Claim waited for that request's transaction to finish.
func (s *TaskService) storedResponse(ctx context.Context, userID, key uuid.UUID, requestHash string) (dto.TaskResponse, error) {
	stored, err := s.idempotency.Find(ctx, userID, key)
	if err != nil {
		return dto.TaskResponse{}, helper.ErrInternal(err)
	}
	if stored.RequestHash != requestHash {
		return dto.TaskResponse{}, helper.NewAppError(http.StatusUnprocessableEntity,
			"IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different request body")
	}
	var res dto.TaskResponse
	if err := json.Unmarshal(stored.ResponseBody, &res); err != nil {
		return dto.TaskResponse{}, helper.ErrInternal(err)
	}
	return res, nil
}

func (s *TaskService) List(ctx context.Context, userID uuid.UUID, q dto.ListTaskQuery) (dto.TaskListResponse, error) {
	filter := model.TaskFilter{
		UserID: userID,
		Search: strings.TrimSpace(q.Search),
		Limit:  q.Limit,
		Offset: (q.Page - 1) * q.Limit,
	}
	if q.Status != "" {
		st := model.TaskStatus(q.Status)
		filter.Status = &st
	}

	tasks, err := s.tasks.List(ctx, filter)
	if err != nil {
		return dto.TaskListResponse{}, helper.ErrInternal(err)
	}
	total, err := s.tasks.Count(ctx, filter)
	if err != nil {
		return dto.TaskListResponse{}, helper.ErrInternal(err)
	}

	data := make([]dto.TaskResponse, len(tasks))
	for i, t := range tasks {
		data[i] = dto.NewTaskResponse(t)
	}
	return dto.TaskListResponse{
		Data:       data,
		Pagination: dto.Pagination{Page: q.Page, Limit: q.Limit, Total: total},
	}, nil
}

func (s *TaskService) Get(ctx context.Context, userID, taskID uuid.UUID) (dto.TaskResponse, error) {
	task, err := s.findVisible(ctx, userID, taskID, false)
	if err != nil {
		return dto.TaskResponse{}, err
	}
	return dto.NewTaskResponse(task), nil
}

// Update replaces the editable fields. Owner only; the assignee can read
// the task but not rewrite it.
func (s *TaskService) Update(ctx context.Context, userID, taskID uuid.UUID, req dto.TaskRequest) (dto.TaskResponse, error) {
	task, err := s.findVisible(ctx, userID, taskID, false)
	if err != nil {
		return dto.TaskResponse{}, err
	}
	if task.OwnerID != userID {
		return dto.TaskResponse{}, helper.ErrForbidden("only the task owner can update it")
	}
	if err := s.ensureTeamMember(ctx, req.TeamID, userID); err != nil {
		return dto.TaskResponse{}, err
	}

	task.Title = strings.TrimSpace(req.Title)
	task.Description = req.Description
	task.Status = statusOrDefault(req.Status)
	task.TeamID = req.TeamID
	if err := s.tasks.Update(ctx, &task); err != nil {
		return dto.TaskResponse{}, helper.ErrInternal(err)
	}
	return dto.NewTaskResponse(task), nil
}

func (s *TaskService) Delete(ctx context.Context, userID, taskID uuid.UUID) error {
	task, err := s.findVisible(ctx, userID, taskID, false)
	if err != nil {
		return err
	}
	if task.OwnerID != userID {
		return helper.ErrForbidden("only the task owner can delete it")
	}
	if err := s.tasks.Delete(ctx, taskID); err != nil {
		return notFoundOr500(err, "task")
	}
	return nil
}

// Assign hands the task to another member of the task's team.
//
// Spec "Database Transaction & Integrity": updating the assignee, writing
// task_logs and sending the notification happen in one transaction. If any
// step fails the closure returns an error and everything is rolled back.
// The notification is the last step so a rollback never follows a message
// that was already sent for a successful write.
func (s *TaskService) Assign(ctx context.Context, userID, taskID uuid.UUID, req dto.AssignTaskRequest) (dto.TaskResponse, error) {
	var task model.Task

	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		task, err = s.findVisible(ctx, userID, taskID, true) // row lock: concurrent assigns queue up
		if err != nil {
			return err
		}
		if task.TeamID == nil {
			return helper.ErrValidation("task has no team, set team_id before assigning")
		}
		if err := s.ensureTeamMember(ctx, task.TeamID, userID); err != nil {
			return err
		}
		if _, err := s.teams.FindMember(ctx, *task.TeamID, req.AssigneeID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return helper.ErrValidation("assignee is not a member of the task's team")
			}
			return helper.ErrInternal(err)
		}

		previous := task.AssigneeID
		task.AssigneeID = &req.AssigneeID
		if err := s.tasks.UpdateAssignee(ctx, &task); err != nil {
			return helper.ErrInternal(err)
		}

		oldValue, _ := json.Marshal(map[string]any{"assignee_id": previous})
		newValue, _ := json.Marshal(map[string]any{"assignee_id": req.AssigneeID})
		if err := s.tasks.CreateLog(ctx, &model.TaskLog{
			TaskID:   task.ID,
			ActorID:  userID,
			Action:   model.TaskActionAssign,
			OldValue: oldValue,
			NewValue: newValue,
		}); err != nil {
			return helper.ErrInternal(err)
		}

		err = s.notifier.NotifyTaskAssigned(ctx, integration.TaskAssignedEvent{
			TaskID:     task.ID,
			TaskTitle:  task.Title,
			AssigneeID: req.AssigneeID,
			AssignedBy: userID,
		})
		if err != nil {
			return &helper.AppError{
				Status:  http.StatusBadGateway,
				Code:    "NOTIFICATION_FAILED",
				Message: "failed to notify assignee, task was not assigned",
				Err:     err,
			}
		}
		return nil
	})
	if err != nil {
		return dto.TaskResponse{}, helper.AsAppError(err)
	}
	return dto.NewTaskResponse(task), nil
}

// findVisible loads a task the user is allowed to see. Tasks that exist but
// belong to someone else return 404, same as missing ones.
func (s *TaskService) findVisible(ctx context.Context, userID, taskID uuid.UUID, forUpdate bool) (model.Task, error) {
	find := s.tasks.FindByID
	if forUpdate {
		find = s.tasks.FindByIDForUpdate
	}
	task, err := find(ctx, taskID)
	if err != nil {
		return model.Task{}, notFoundOr500(err, "task")
	}
	if !task.IsVisibleTo(userID) {
		return model.Task{}, helper.ErrNotFound("task")
	}
	return task, nil
}

func (s *TaskService) ensureTeamMember(ctx context.Context, teamID *uuid.UUID, userID uuid.UUID) error {
	if teamID == nil {
		return nil
	}
	_, err := s.teams.FindMember(ctx, *teamID, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return helper.ErrForbidden("you are not a member of this team")
	}
	if err != nil {
		return helper.ErrInternal(err)
	}
	return nil
}

// hashRequest fingerprints the parsed request, so the same key can't be
// reused for a different task. Hashing the parsed struct instead of raw
// bytes means formatting differences (spaces, key order) don't count.
func hashRequest(req dto.TaskRequest) (string, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func statusOrDefault(s string) model.TaskStatus {
	if s == "" {
		return model.TaskStatusTodo
	}
	return model.TaskStatus(s)
}
