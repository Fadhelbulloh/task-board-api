package helper

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError is the only error type that should cross the service ->
// controller boundary. Status and Code go to the client; Err is logged only.
type AppError struct {
	Status  int
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return e.Code + ": " + e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func NewAppError(status int, code, msg string) *AppError {
	return &AppError{Status: status, Code: code, Message: msg}
}

func ErrBadRequest(msg string) *AppError {
	return NewAppError(http.StatusBadRequest, "BAD_REQUEST", msg)
}

func ErrValidation(msg string) *AppError {
	return NewAppError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", msg)
}

func ErrUnauthorized(msg string) *AppError {
	return NewAppError(http.StatusUnauthorized, "UNAUTHORIZED", msg)
}

func ErrForbidden(msg string) *AppError {
	return NewAppError(http.StatusForbidden, "FORBIDDEN", msg)
}

func ErrNotFound(resource string) *AppError {
	return NewAppError(http.StatusNotFound, "NOT_FOUND", resource+" not found")
}

func ErrConflict(code, msg string) *AppError {
	return NewAppError(http.StatusConflict, code, msg)
}

// ErrInternal hides err behind a generic message. Use for anything the
// client can't act on (DB down, bug, etc).
func ErrInternal(err error) *AppError {
	return &AppError{Status: http.StatusInternalServerError, Code: "INTERNAL_ERROR", Message: "internal server error", Err: err}
}

// AsAppError turns any error into an AppError; unknown errors become 500.
func AsAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return ErrInternal(err)
}
