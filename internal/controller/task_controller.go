package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/dto"
	"github.com/Fadhelbulloh/task-board-api/internal/helper"
	"github.com/Fadhelbulloh/task-board-api/internal/service"
)

type TaskController struct {
	taskService *service.TaskService
}

func NewTaskController(taskService *service.TaskService) *TaskController {
	return &TaskController{taskService: taskService}
}

// POST /tasks
// Requires an Idempotency-Key header (UUID). A repeat with the same key and
// body gets the original response plus "Idempotent-Replayed: true".
func (ctl *TaskController) Create(c *gin.Context) {
	key, err := uuid.Parse(c.GetHeader("Idempotency-Key"))
	if err != nil {
		helper.RespondError(c, helper.ErrBadRequest("Idempotency-Key header is required and must be a UUID"))
		return
	}
	var req dto.TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondError(c, helper.BindError(err))
		return
	}
	res, replayed, err := ctl.taskService.Create(c.Request.Context(), helper.GetUserID(c), key, req)
	if err != nil {
		helper.RespondError(c, err)
		return
	}
	if replayed {
		c.Header("Idempotent-Replayed", "true")
	}
	c.JSON(http.StatusCreated, res)
}

// GET /tasks?status=&search=&page=&limit=
func (ctl *TaskController) List(c *gin.Context) {
	q := dto.ListTaskQuery{Page: 1, Limit: 20}
	if err := c.ShouldBindQuery(&q); err != nil {
		helper.RespondError(c, helper.BindError(err))
		return
	}
	res, err := ctl.taskService.List(c.Request.Context(), helper.GetUserID(c), q)
	if err != nil {
		helper.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// GET /tasks/:id
func (ctl *TaskController) Get(c *gin.Context) {
	taskID, ok := helper.ParamUUID(c, "id")
	if !ok {
		return
	}
	res, err := ctl.taskService.Get(c.Request.Context(), helper.GetUserID(c), taskID)
	if err != nil {
		helper.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// PUT /tasks/:id
func (ctl *TaskController) Update(c *gin.Context) {
	taskID, ok := helper.ParamUUID(c, "id")
	if !ok {
		return
	}
	var req dto.TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondError(c, helper.BindError(err))
		return
	}
	res, err := ctl.taskService.Update(c.Request.Context(), helper.GetUserID(c), taskID, req)
	if err != nil {
		helper.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// DELETE /tasks/:id
func (ctl *TaskController) Delete(c *gin.Context) {
	taskID, ok := helper.ParamUUID(c, "id")
	if !ok {
		return
	}
	if err := ctl.taskService.Delete(c.Request.Context(), helper.GetUserID(c), taskID); err != nil {
		helper.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// POST /tasks/:id/assign
func (ctl *TaskController) Assign(c *gin.Context) {
	taskID, ok := helper.ParamUUID(c, "id")
	if !ok {
		return
	}
	var req dto.AssignTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondError(c, helper.BindError(err))
		return
	}
	res, err := ctl.taskService.Assign(c.Request.Context(), helper.GetUserID(c), taskID, req)
	if err != nil {
		helper.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}
