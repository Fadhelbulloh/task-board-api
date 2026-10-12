// Package route is the map of the API: every endpoint and the middleware in
// front of it. Start here to follow any request.
package route

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Fadhelbulloh/task-board-api/internal/controller"
	"github.com/Fadhelbulloh/task-board-api/internal/helper"
	"github.com/Fadhelbulloh/task-board-api/internal/middleware"
)

// maxBodyBytes is far above any real payload (a task is a few KB) and low
// enough that nobody can make the server buffer something huge.
const maxBodyBytes = 1 << 20

type Dependencies struct {
	JWT            *helper.JWT
	AuthController *controller.AuthController
	TeamController *controller.TeamController
	TaskController *controller.TaskController
}

func New(d Dependencies) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true

	// RequestID first so Recovery and Logger can include it.
	r.Use(middleware.RequestID(), middleware.Recovery(), middleware.Logger(), middleware.BodyLimit(maxBodyBytes))

	r.NoRoute(func(c *gin.Context) { helper.RespondError(c, helper.ErrNotFound("route")) })
	r.NoMethod(func(c *gin.Context) {
		helper.RespondError(c, helper.NewAppError(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed"))
	})

	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	auth := r.Group("/auth")
	auth.POST("/register", d.AuthController.Register)
	auth.POST("/login", d.AuthController.Login)

	protected := r.Group("/", middleware.Auth(d.JWT))

	tasks := protected.Group("/tasks")
	tasks.POST("", d.TaskController.Create) // requires Idempotency-Key header
	tasks.GET("", d.TaskController.List)
	tasks.GET("/:id", d.TaskController.Get)
	tasks.PUT("/:id", d.TaskController.Update)
	tasks.DELETE("/:id", d.TaskController.Delete)
	tasks.POST("/:id/assign", d.TaskController.Assign)

	teams := protected.Group("/teams")
	teams.POST("", d.TeamController.Create)
	teams.GET("", d.TeamController.List)
	teams.GET("/:id", d.TeamController.Get)
	teams.PUT("/:id", d.TeamController.Update)
	teams.DELETE("/:id", d.TeamController.Delete)
	teams.GET("/:id/members", d.TeamController.ListMembers)
	teams.POST("/:id/members", d.TeamController.AddMember)
	teams.DELETE("/:id/members/:userId", d.TeamController.RemoveMember)

	return r
}
