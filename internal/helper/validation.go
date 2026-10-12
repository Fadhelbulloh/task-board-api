package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Report json/form tag names in validation errors ("title", not "Title").
func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(f reflect.StructField) string {
			for _, tag := range []string{"json", "form"} {
				name, _, _ := strings.Cut(f.Tag.Get(tag), ",")
				if name != "" && name != "-" {
					return name
				}
			}
			return f.Name
		})
	}
}

// BindError turns a ShouldBind* error into a client-friendly AppError, so
// Go type and struct names never reach the response.
func BindError(err error) *AppError {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return NewAppError(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE",
			fmt.Sprintf("request body must not exceed %d bytes", maxBytesErr.Limit))
	}

	var fieldErrs validator.ValidationErrors
	if errors.As(err, &fieldErrs) {
		msgs := make([]string, len(fieldErrs))
		for i, fe := range fieldErrs {
			msgs[i] = fieldMessage(fe)
		}
		return ErrValidation(strings.Join(msgs, "; "))
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return ErrValidation(fmt.Sprintf("%s has the wrong type, expected %s", typeErr.Field, typeErr.Type))
	}

	// Query binding: ?limit=abc
	var numErr *strconv.NumError
	if errors.As(err, &numErr) {
		return ErrBadRequest(fmt.Sprintf("%q is not a valid number", numErr.Num))
	}

	// Malformed JSON, empty body, bad UUID in body, etc.
	return ErrBadRequest("request body is not valid JSON")
}

func fieldMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fe.Field() + " is required"
	case "email":
		return fe.Field() + " must be a valid email"
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", fe.Field(), strings.ReplaceAll(fe.Param(), " ", ", "))
	case "min":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("%s must be at least %s characters", fe.Field(), fe.Param())
		}
		return fmt.Sprintf("%s must be at least %s", fe.Field(), fe.Param())
	case "max":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("%s must be at most %s characters", fe.Field(), fe.Param())
		}
		return fmt.Sprintf("%s must be at most %s", fe.Field(), fe.Param())
	default:
		return fe.Field() + " is invalid"
	}
}
