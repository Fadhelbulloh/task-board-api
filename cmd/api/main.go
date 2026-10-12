package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Fadhelbulloh/task-board-api/internal/config"
	"github.com/Fadhelbulloh/task-board-api/internal/controller"
	"github.com/Fadhelbulloh/task-board-api/internal/database"
	"github.com/Fadhelbulloh/task-board-api/internal/helper"
	"github.com/Fadhelbulloh/task-board-api/internal/integration"
	"github.com/Fadhelbulloh/task-board-api/internal/repository"
	"github.com/Fadhelbulloh/task-board-api/internal/route"
	"github.com/Fadhelbulloh/task-board-api/internal/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.Migrate(pool); err != nil {
		return err
	}

	// repository -> service -> controller -> route
	transactor := repository.NewTransactor(pool)
	userRepo := repository.NewUserRepository(pool)
	teamRepo := repository.NewTeamRepository(pool)
	taskRepo := repository.NewTaskRepository(pool)
	idempotencyRepo := repository.NewIdempotencyRepository(pool)

	jwt := helper.NewJWT(cfg.JWTSecret, cfg.JWTTTL)

	authService := service.NewAuthService(userRepo, jwt)
	teamService := service.NewTeamService(transactor, teamRepo, userRepo)
	taskService := service.NewTaskService(transactor, taskRepo, teamRepo, idempotencyRepo, cfg.IdempotencyTTL, integration.NewLogNotifier())

	router := route.New(route.Dependencies{
		JWT:            jwt,
		AuthController: controller.NewAuthController(authService),
		TeamController: controller.NewTeamController(teamService),
		TaskController: controller.NewTaskController(taskService),
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server started", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
