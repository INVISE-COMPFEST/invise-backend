package errors_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	apperrors "invise-backend/pkg/errors"
)

func TestAppError(t *testing.T) {
	t.Run("Error returns message", func(t *testing.T) {
		err := apperrors.New(http.StatusBadRequest, "BAD_REQUEST", "bad request message", nil)
		assert.Equal(t, "bad request message", err.Error())
	})

	t.Run("Unwrap returns wrapped error", func(t *testing.T) {
		underlying := errors.New("underlying error")
		err := apperrors.Wrap(http.StatusInternalServerError, "INTERNAL_ERROR", "wrapped error", underlying)
		assert.Equal(t, underlying, err.Unwrap())
		assert.True(t, errors.Is(err, underlying))
	})

	t.Run("Constructors", func(t *testing.T) {
		tests := []struct {
			name       string
			err        *apperrors.AppError
			statusCode int
			code       string
			message    string
		}{
			{
				name:       "BadRequest",
				err:        apperrors.BadRequest("INVALID_REQ", "invalid"),
				statusCode: http.StatusBadRequest,
				code:       "INVALID_REQ",
				message:    "invalid",
			},
			{
				name:       "Unauthorized",
				err:        apperrors.Unauthorized("UNAUTHORIZED", "unauthorized"),
				statusCode: http.StatusUnauthorized,
				code:       "UNAUTHORIZED",
				message:    "unauthorized",
			},
			{
				name:       "Forbidden",
				err:        apperrors.Forbidden("FORBIDDEN", "forbidden"),
				statusCode: http.StatusForbidden,
				code:       "FORBIDDEN",
				message:    "forbidden",
			},
			{
				name:       "NotFound",
				err:        apperrors.NotFound("NOT_FOUND", "not found"),
				statusCode: http.StatusNotFound,
				code:       "NOT_FOUND",
				message:    "not found",
			},
			{
				name:       "Conflict",
				err:        apperrors.Conflict("CONFLICT", "conflict"),
				statusCode: http.StatusConflict,
				code:       "CONFLICT",
				message:    "conflict",
			},
			{
				name:       "InternalServerError",
				err:        apperrors.InternalServerError("INTERNAL", "internal error"),
				statusCode: http.StatusInternalServerError,
				code:       "INTERNAL",
				message:    "internal error",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.statusCode, tt.err.StatusCode)
				assert.Equal(t, tt.code, tt.err.Code)
				assert.Equal(t, tt.message, tt.err.Message)
				assert.Nil(t, tt.err.Details)
			})
		}
	})

	t.Run("Global sentinel errors", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, apperrors.ErrInvalidRequest.StatusCode)
		assert.Equal(t, "INVALID_REQUEST", apperrors.ErrInvalidRequest.Code)
		assert.Equal(t, "invalid request body", apperrors.ErrInvalidRequest.Message)

		assert.Equal(t, http.StatusBadRequest, apperrors.ErrValidationError.StatusCode)
		assert.Equal(t, "VALIDATION_ERROR", apperrors.ErrValidationError.Code)
		assert.Equal(t, "validation error", apperrors.ErrValidationError.Message)

		assert.Equal(t, http.StatusInternalServerError, apperrors.ErrInternalServerError.StatusCode)
		assert.Equal(t, "INTERNAL_ERROR", apperrors.ErrInternalServerError.Code)

		assert.Equal(t, http.StatusUnauthorized, apperrors.ErrUnauthorized.StatusCode)
		assert.Equal(t, "UNAUTHORIZED", apperrors.ErrUnauthorized.Code)

		assert.Equal(t, http.StatusForbidden, apperrors.ErrForbidden.StatusCode)
		assert.Equal(t, "FORBIDDEN", apperrors.ErrForbidden.Code)

		assert.Equal(t, http.StatusNotFound, apperrors.ErrNotFound.StatusCode)
		assert.Equal(t, "NOT_FOUND", apperrors.ErrNotFound.Code)
	})
}
