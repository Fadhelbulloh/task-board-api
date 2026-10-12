package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Fadhelbulloh/task-board-api/internal/dto"
	"github.com/Fadhelbulloh/task-board-api/internal/helper"
	"github.com/Fadhelbulloh/task-board-api/internal/service"
)

type AuthController struct {
	authService *service.AuthService
}

func NewAuthController(authService *service.AuthService) *AuthController {
	return &AuthController{authService: authService}
}

// POST /auth/register
func (ctl *AuthController) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondError(c, helper.BindError(err))
		return
	}
	res, err := ctl.authService.Register(c.Request.Context(), req)
	if err != nil {
		helper.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

// POST /auth/login
func (ctl *AuthController) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondError(c, helper.BindError(err))
		return
	}
	res, err := ctl.authService.Login(c.Request.Context(), req)
	if err != nil {
		helper.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}
