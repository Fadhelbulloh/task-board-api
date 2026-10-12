package route

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/controller"
	"github.com/Fadhelbulloh/task-board-api/internal/dto"
	"github.com/Fadhelbulloh/task-board-api/internal/helper"
	"github.com/Fadhelbulloh/task-board-api/internal/integration"
	"github.com/Fadhelbulloh/task-board-api/internal/mock"
	"github.com/Fadhelbulloh/task-board-api/internal/service"
)

// testApp wires the real router, middleware, controllers and services on
// top of in-memory repositories.
type testApp struct {
	router *gin.Engine
	db     *mock.DB
	tasks  *mock.TaskRepository
	idem   *mock.IdempotencyRepository
	token  string
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := mock.NewDB()
	jwt := helper.NewJWT("test-secret-test-secret-test-secret", time.Hour)
	tx := mock.Transactor{DB: db}
	users := mock.UserRepository{DB: db}
	teams := mock.TeamRepository{DB: db}
	tasks := &mock.TaskRepository{DB: db}
	idem := &mock.IdempotencyRepository{DB: db}

	router := New(Dependencies{
		JWT:            jwt,
		AuthController: controller.NewAuthController(service.NewAuthService(users, jwt)),
		TeamController: controller.NewTeamController(service.NewTeamService(tx, teams, users)),
		TaskController: controller.NewTaskController(service.NewTaskService(tx, tasks, teams, idem, 24*time.Hour, integration.NewLogNotifier())),
	})

	token, _, err := jwt.Generate(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	return &testApp{router: router, db: db, tasks: tasks, idem: idem, token: token}
}

func (a *testApp) do(method, path, key, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.token)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return rec
}

func (a *testApp) createTask(key, body string) *httptest.ResponseRecorder {
	return a.do(http.MethodPost, "/tasks", key, body)
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var res dto.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("error body is not JSON: %s", rec.Body)
	}
	return res.Code
}

// Spec 5.1 - Sequential: first request creates (201), second with the same
// key returns the identical response and creates nothing.
func TestIdempotency_Sequential(t *testing.T) {
	app := newTestApp(t)
	key := uuid.NewString()
	body := `{"title":"Write report"}`

	first := app.createTask(key, body)
	if first.Code != http.StatusCreated {
		t.Fatalf("first request: got %d, want 201. body=%s", first.Code, first.Body)
	}

	second := app.createTask(key, body)
	if second.Code != http.StatusCreated {
		t.Fatalf("second request: got %d, want 201. body=%s", second.Code, second.Body)
	}
	if second.Body.String() != first.Body.String() {
		t.Fatalf("replayed body differs\nfirst:  %s\nsecond: %s", first.Body, second.Body)
	}
	if first.Header().Get("Idempotent-Replayed") != "" {
		t.Error("first response must not be marked as replayed")
	}
	if second.Header().Get("Idempotent-Replayed") != "true" {
		t.Error("second response should carry Idempotent-Replayed: true")
	}
	if n := app.db.TaskCount(); n != 1 {
		t.Fatalf("tasks in db = %d, want 1", n)
	}
}

// Spec 5.1 - Concurrent duplicate: N goroutines, same key, released at the
// same instant. Exactly one task may exist afterwards, and every caller
// gets the same 201 because later requests wait for the first to commit.
func TestIdempotency_ConcurrentDuplicate(t *testing.T) {
	const workers = 50
	app := newTestApp(t)
	// Keep the winner inside its transaction long enough for the others to
	// pile up behind it, so the requests genuinely overlap.
	app.tasks.CreateDelay = 30 * time.Millisecond
	key := uuid.NewString()
	body := `{"title":"Concurrent task"}`

	responses := make([]*httptest.ResponseRecorder, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			responses[i] = app.createTask(key, body)
		}()
	}
	close(start)
	wg.Wait()

	if n := app.db.TaskCount(); n != 1 {
		t.Fatalf("tasks in db = %d, want exactly 1", n)
	}

	want := responses[0].Body.String()
	var replayed int
	for i, rec := range responses {
		if rec.Code != http.StatusCreated {
			t.Fatalf("response %d: status %d, body %s", i, rec.Code, rec.Body)
		}
		if rec.Body.String() != want {
			t.Fatalf("response %d differs:\n%s\n%s", i, want, rec.Body)
		}
		if rec.Header().Get("Idempotent-Replayed") == "true" {
			replayed++
		}
	}
	if replayed != workers-1 {
		t.Fatalf("replayed = %d, want %d (all but the one that created the task)", replayed, workers-1)
	}
}

func TestIdempotency_SameKeyDifferentBody(t *testing.T) {
	app := newTestApp(t)
	key := uuid.NewString()

	app.createTask(key, `{"title":"A"}`)
	rec := app.createTask(key, `{"title":"B"}`)

	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("got %d %s, want 422 IDEMPOTENCY_KEY_REUSED", rec.Code, rec.Body)
	}
	if n := app.db.TaskCount(); n != 1 {
		t.Fatalf("tasks in db = %d, want 1", n)
	}
}

// Formatting doesn't change the request: same fields, different whitespace
// and key order is still a replay, not a conflict.
func TestIdempotency_IgnoresJSONFormatting(t *testing.T) {
	app := newTestApp(t)
	key := uuid.NewString()

	app.createTask(key, `{"title":"A","status":"done"}`)
	rec := app.createTask(key, "{ \"status\": \"done\",\n  \"title\": \"A\" }")

	if rec.Code != http.StatusCreated || rec.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("got %d %s, want replayed 201", rec.Code, rec.Body)
	}
}

func TestIdempotency_HeaderRequired(t *testing.T) {
	app := newTestApp(t)
	for _, key := range []string{"", "abc"} {
		if rec := app.createTask(key, `{"title":"A"}`); rec.Code != http.StatusBadRequest {
			t.Errorf("key %q: got %d, want 400", key, rec.Code)
		}
	}
	if n := app.db.TaskCount(); n != 0 {
		t.Fatalf("tasks in db = %d, want 0", n)
	}
}

// If storing the response fails, the task insert must roll back with it.
// Otherwise the retry would find no key and create a second task.
func TestIdempotency_NoDuplicateWhenSavingResponseFails(t *testing.T) {
	app := newTestApp(t)
	key := uuid.NewString()
	body := `{"title":"A"}`

	app.idem.SaveResponseErr = errors.New("connection reset")
	if rec := app.createTask(key, body); rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", rec.Code)
	}
	if n, k := app.db.TaskCount(), app.db.IdempotencyKeyCount(); n != 0 || k != 0 {
		t.Fatalf("after failure: tasks=%d keys=%d, want 0 and 0", n, k)
	}

	app.idem.SaveResponseErr = nil
	if rec := app.createTask(key, body); rec.Code != http.StatusCreated {
		t.Fatalf("retry: got %d %s, want 201", rec.Code, rec.Body)
	}
	if n := app.db.TaskCount(); n != 1 {
		t.Fatalf("tasks in db = %d, want 1", n)
	}
}

// A request that fails inside the transaction doesn't burn the key: once
// the cause is fixed, the client can retry with the same key.
func TestIdempotency_FailedRequestDoesNotConsumeKey(t *testing.T) {
	app := newTestApp(t)
	key := uuid.NewString()
	teamID := uuid.New()
	body := `{"title":"A","team_id":"` + teamID.String() + `"}`

	if rec := app.createTask(key, body); rec.Code != http.StatusForbidden {
		t.Fatalf("got %d %s, want 403 (not a team member)", rec.Code, rec.Body)
	}
	if k := app.db.IdempotencyKeyCount(); k != 0 {
		t.Fatalf("key stored after a failed request")
	}
}

func TestValidation_MessagesUseJSONFieldNames(t *testing.T) {
	app := newTestApp(t)

	tests := []struct {
		name, method, path, body string
		wantStatus               int
		wantMessage              string
	}{
		{"missing title", http.MethodPost, "/tasks", `{}`, 422, "title is required"},
		{"bad status", http.MethodPost, "/tasks", `{"title":"A","status":"later"}`, 422, "status must be one of: todo, in_progress, done"},
		{"title too long", http.MethodPost, "/tasks", `{"title":"` + strings.Repeat("x", 201) + `"}`, 422, "title must be at most 200 characters"},
		{"wrong type", http.MethodPost, "/tasks", `{"title":123}`, 422, "title has the wrong type, expected string"},
		{"malformed json", http.MethodPost, "/tasks", `{"title":`, 400, "request body is not valid JSON"},
		{"limit too big", http.MethodGet, "/tasks?limit=500", ``, 422, "limit must be at most 100"},
		{"limit not a number", http.MethodGet, "/tasks?limit=abc", ``, 400, `"abc" is not a valid number`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := app.do(tc.method, tc.path, uuid.NewString(), tc.body)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d. body=%s", rec.Code, tc.wantStatus, rec.Body)
			}
			var res dto.ErrorResponse
			_ = json.Unmarshal(rec.Body.Bytes(), &res)
			if res.Message != tc.wantMessage {
				t.Fatalf("message = %q, want %q", res.Message, tc.wantMessage)
			}
		})
	}
}

func TestBodyLimit(t *testing.T) {
	app := newTestApp(t)
	huge := `{"title":"A","description":"` + strings.Repeat("x", maxBodyBytes) + `"}`

	rec := app.createTask(uuid.NewString(), huge)
	if rec.Code != http.StatusRequestEntityTooLarge || errorCode(t, rec) != "PAYLOAD_TOO_LARGE" {
		t.Fatalf("got %d %s, want 413 PAYLOAD_TOO_LARGE", rec.Code, rec.Body)
	}
}
