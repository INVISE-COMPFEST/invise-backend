package errors

import "net/http"

type AppError struct {
	StatusCode int         `json:"-"`
	Code       string      `json:"code"`
	Message    string      `json:"message"`
	Details    interface{} `json:"details,omitempty"`
	Err        error       `json:"-"`
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Err }

func New(statusCode int, code, message string, details interface{}) *AppError {
	return &AppError{StatusCode: statusCode, Code: code, Message: message, Details: details}
}

func Wrap(statusCode int, code, message string, err error) *AppError {
	return &AppError{StatusCode: statusCode, Code: code, Message: message, Err: err}
}

func BadRequest(code, message string) *AppError {
	return New(http.StatusBadRequest, code, message, nil)
}

func Unauthorized(code, message string) *AppError {
	return New(http.StatusUnauthorized, code, message, nil)
}

func Forbidden(code, message string) *AppError {
	return New(http.StatusForbidden, code, message, nil)
}

func NotFound(code, message string) *AppError {
	return New(http.StatusNotFound, code, message, nil)
}

func Conflict(code, message string) *AppError {
	return New(http.StatusConflict, code, message, nil)
}

func InternalServerError(code, message string) *AppError {
	return New(http.StatusInternalServerError, code, message, nil)
}

func BadGateway(code, message string) *AppError {
	return New(http.StatusBadGateway, code, message, nil)
}

func ServiceUnavailable(code, message string) *AppError {
	return New(http.StatusServiceUnavailable, code, message, nil)
}

// Global sentinel errors
var (
	ErrInvalidRequest      = BadRequest("INVALID_REQUEST", "invalid request body")
	ErrValidationError     = BadRequest("VALIDATION_ERROR", "validation error")
	ErrInternalServerError = InternalServerError("INTERNAL_ERROR", "internal server error")
	ErrUnauthorized        = Unauthorized("UNAUTHORIZED", "unauthorized")
	ErrForbidden           = Forbidden("FORBIDDEN", "forbidden")
	ErrNotFound            = NotFound("NOT_FOUND", "not found")
)
