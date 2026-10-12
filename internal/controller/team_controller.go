package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Fadhelbulloh/task-board-api/internal/dto"
	"github.com/Fadhelbulloh/task-board-api/internal/helper"
	"github.com/Fadhelbulloh/task-board-api/internal/service"
)

type TeamController struct {
	teamService *service.TeamService
}

func NewTeamController(teamService *service.TeamService) *TeamController {
	return &TeamController{teamService: teamService}
}

// POST /teams
func (ctl *TeamController) Create(c *gin.Context) {
	var req dto.TeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondError(c, helper.BindError(err))
		return
	}
	res, err := ctl.teamService.Create(c.Request.Context(), helper.GetUserID(c), req)
	if err != nil {
		helper.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

// GET /teams
func (ctl *TeamController) List(c *gin.Context) {
	res, err := ctl.teamService.List(c.Request.Context(), helper.GetUserID(c))
	if err != nil {
		helper.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

// GET /teams/:id
func (ctl *TeamController) Get(c *gin.Context) {
	teamID, ok := helper.ParamUUID(c, "id")
	if !ok {
		return
	}
	res, err := ctl.teamService.Get(c.Request.Context(), helper.GetUserID(c), teamID)
	if err != nil {
		helper.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// PUT /teams/:id
func (ctl *TeamController) Update(c *gin.Context) {
	teamID, ok := helper.ParamUUID(c, "id")
	if !ok {
		return
	}
	var req dto.TeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondError(c, helper.BindError(err))
		return
	}
	res, err := ctl.teamService.Update(c.Request.Context(), helper.GetUserID(c), teamID, req)
	if err != nil {
		helper.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// DELETE /teams/:id
func (ctl *TeamController) Delete(c *gin.Context) {
	teamID, ok := helper.ParamUUID(c, "id")
	if !ok {
		return
	}
	if err := ctl.teamService.Delete(c.Request.Context(), helper.GetUserID(c), teamID); err != nil {
		helper.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GET /teams/:id/members
func (ctl *TeamController) ListMembers(c *gin.Context) {
	teamID, ok := helper.ParamUUID(c, "id")
	if !ok {
		return
	}
	res, err := ctl.teamService.ListMembers(c.Request.Context(), helper.GetUserID(c), teamID)
	if err != nil {
		helper.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

// POST /teams/:id/members
func (ctl *TeamController) AddMember(c *gin.Context) {
	teamID, ok := helper.ParamUUID(c, "id")
	if !ok {
		return
	}
	var req dto.AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondError(c, helper.BindError(err))
		return
	}
	if err := ctl.teamService.AddMember(c.Request.Context(), helper.GetUserID(c), teamID, req); err != nil {
		helper.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DELETE /teams/:id/members/:userId
func (ctl *TeamController) RemoveMember(c *gin.Context) {
	teamID, ok := helper.ParamUUID(c, "id")
	if !ok {
		return
	}
	memberID, ok := helper.ParamUUID(c, "userId")
	if !ok {
		return
	}
	if err := ctl.teamService.RemoveMember(c.Request.Context(), helper.GetUserID(c), teamID, memberID); err != nil {
		helper.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
