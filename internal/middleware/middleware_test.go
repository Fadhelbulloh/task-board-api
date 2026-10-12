package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Fadhelbulloh/task-board-api/internal/dto"
	"github.com/Fadhelbulloh/task-board-api/internal/helper"
)

// captureLogs redirects the default slog logger for the duration of a test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestRecovery_HidesPanicFromClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logs := captureLogs(t)

	r := gin.New()
	r.Use(RequestID(), Recovery(), Logger())
	r.GET("/panic", func(*gin.Context) { panic("nil map write in secret_module.go") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var res dto.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Code != "INTERNAL_ERROR" || res.Timestamp == "" || res.RequestID == "" {
		t.Fatalf("unexpected body: %+v", res)
	}
	if strings.Contains(rec.Body.String(), "secret_module") {
		t.Fatal("panic detail leaked to the response")
	}
	if !strings.Contains(logs.String(), "secret_module") {
		t.Fatal("panic detail missing from logs")
	}
}

func TestLogger_LevelByStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := map[int]string{200: "INFO", 404: "WARN", 503: "ERROR"}

	for status, wantLevel := range cases {
		logs := captureLogs(t)
		r := gin.New()
		r.Use(RequestID(), Logger())
		r.GET("/", func(c *gin.Context) { c.Status(status) })
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

		var entry map[string]any
		if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
			t.Fatalf("log line is not JSON: %q", logs.String())
		}
		if entry["level"] != wantLevel {
			t.Errorf("status %d logged at %v, want %s", status, entry["level"], wantLevel)
		}
		for _, field := range []string{"request_id", "method", "path", "status", "latency_ms"} {
			if _, ok := entry[field]; !ok {
				t.Errorf("status %d: log missing %q", status, field)
			}
		}
	}
}

func TestAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-test-secret-test-secret"
	jwt := helper.NewJWT(secret, time.Minute)
	userID := uuid.New()

	valid, _, _ := jwt.Generate(userID)
	expired, _, _ := helper.NewJWT(secret, -time.Minute).Generate(userID)
	wrongKey, _, _ := helper.NewJWT("another-secret-another-secret-xx", time.Minute).Generate(userID)

	r := gin.New()
	r.Use(RequestID())
	r.GET("/me", Auth(jwt), func(c *gin.Context) { c.String(http.StatusOK, helper.GetUserID(c).String()) })

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic " + valid, http.StatusUnauthorized},
		{"garbage token", "Bearer abc.def.ghi", http.StatusUnauthorized},
		{"expired", "Bearer " + expired, http.StatusUnauthorized},
		{"signed with another key", "Bearer " + wrongKey, http.StatusUnauthorized},
		{"valid", "Bearer " + valid, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if tc.want == http.StatusOK && rec.Body.String() != userID.String() {
				t.Fatalf("user id = %s, want %s", rec.Body, userID)
			}
		})
	}
}
