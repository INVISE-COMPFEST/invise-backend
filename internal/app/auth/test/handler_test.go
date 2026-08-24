package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"

	"invise-backend/internal/app/auth"
	pkgerr "invise-backend/pkg/errors"
	"invise-backend/pkg/response"
)

type stubAuthUsecase struct {
	registerFn func(ctx context.Context, req auth.RegisterRequest) error
	verifyFn   func(ctx context.Context, req auth.VerifyRequest) error
	loginFn    func(ctx context.Context, req auth.LoginRequest) (*auth.TokenResponse, error)
}

func (s *stubAuthUsecase) Register(ctx context.Context, req auth.RegisterRequest) error {
	if s.registerFn != nil {
		return s.registerFn(ctx, req)
	}
	return nil
}

func (s *stubAuthUsecase) Verify(ctx context.Context, req auth.VerifyRequest) error {
	if s.verifyFn != nil {
		return s.verifyFn(ctx, req)
	}
	return nil
}

func (s *stubAuthUsecase) Login(ctx context.Context, req auth.LoginRequest) (*auth.TokenResponse, error) {
	if s.loginFn != nil {
		return s.loginFn(ctx, req)
	}
	return nil, nil
}

func testErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "internal server error"

	var appErr *pkgerr.AppError
	if errors.As(err, &appErr) {
		code = appErr.StatusCode
		message = appErr.Message
	}

	return c.Status(code).JSON(dto.Response[any]{
		Message: message,
	})
}

func setupHandlerTestApp(h *auth.AuthHandler) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: testErrorHandler,
	})
	app.Post("/register", h.Register)
	app.Post("/verify", h.Verify)
	app.Post("/login", h.Login)
	return app
}

func TestAuthHandler_Register(t *testing.T) {
	stubUc := &stubAuthUsecase{
		registerFn: func(ctx context.Context, req auth.RegisterRequest) error {
			assert.Equal(t, "test@example.com", req.Email)
			assert.Equal(t, "password123", req.Password)
			return nil
		},
	}
	v := validator.New()
	handler := auth.NewAuthHandler(stubUc, v)
	app := setupHandlerTestApp(handler)

	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	})
	req, _ := http.NewRequest("POST", "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)

	var resBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&resBody)
	assert.NoError(t, err)
	assert.Equal(t, "registration successful, please check your email for verification code", resBody["message"])
	assert.Nil(t, resBody["data"])
}

func TestAuthHandler_Verify(t *testing.T) {
	stubUc := &stubAuthUsecase{
		verifyFn: func(ctx context.Context, req auth.VerifyRequest) error {
			assert.Equal(t, "test@example.com", req.Email)
			assert.Equal(t, "123456", req.OTP)
			return nil
		},
	}
	v := validator.New()
	handler := auth.NewAuthHandler(stubUc, v)
	app := setupHandlerTestApp(handler)

	body, _ := json.Marshal(map[string]string{
		"email": "test@example.com",
		"otp":   "123456",
	})
	req, _ := http.NewRequest("POST", "/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var resBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&resBody)
	assert.NoError(t, err)
	assert.Equal(t, "account verified successfully", resBody["message"])
	assert.Nil(t, resBody["data"])
}

func TestAuthHandler_Login(t *testing.T) {
	stubUc := &stubAuthUsecase{
		loginFn: func(ctx context.Context, req auth.LoginRequest) (*auth.TokenResponse, error) {
			assert.Equal(t, "test@example.com", req.Email)
			assert.Equal(t, "password123", req.Password)
			return &auth.TokenResponse{AccessToken: "token-abc-123"}, nil
		},
	}
	v := validator.New()
	handler := auth.NewAuthHandler(stubUc, v)
	app := setupHandlerTestApp(handler)

	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	})
	req, _ := http.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var resBody struct {
		Message string `json:"message"`
		Data    struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&resBody)
	assert.NoError(t, err)
	assert.Equal(t, "login successful", resBody.Message)
	assert.Equal(t, "token-abc-123", resBody.Data.AccessToken)
}

func TestAuthHandler_Register_Errors(t *testing.T) {
	v := validator.New()
	handler := auth.NewAuthHandler(&stubAuthUsecase{}, v)
	app := setupHandlerTestApp(handler)

	t.Run("Invalid Request Body", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/register", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

		var resBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&resBody)
		assert.Equal(t, "invalid request body", resBody["message"])
	})

	t.Run("Validation Error", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email": "not-an-email",
		})
		req, _ := http.NewRequest("POST", "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

		var resBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&resBody)
		assert.Equal(t, "validation error", resBody["message"])
	})

	t.Run("Usecase Error", func(t *testing.T) {
		uc := &stubAuthUsecase{
			registerFn: func(ctx context.Context, req auth.RegisterRequest) error {
				return auth.ErrEmailAlreadyExists
			},
		}
		h := auth.NewAuthHandler(uc, v)
		app := setupHandlerTestApp(h)

		body, _ := json.Marshal(map[string]string{
			"email":    "test@example.com",
			"password": "password123",
		})
		req, _ := http.NewRequest("POST", "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusConflict, resp.StatusCode)

		var resBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&resBody)
		assert.Equal(t, "email is already registered", resBody["message"])
	})
}

func TestAuthHandler_Verify_Errors(t *testing.T) {
	v := validator.New()
	handler := auth.NewAuthHandler(&stubAuthUsecase{}, v)
	app := setupHandlerTestApp(handler)

	t.Run("Invalid Request Body", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/verify", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

		var resBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&resBody)
		assert.Equal(t, "invalid request body", resBody["message"])
	})

	t.Run("Validation Error", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email": "invalid-email",
		})
		req, _ := http.NewRequest("POST", "/verify", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

		var resBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&resBody)
		assert.Equal(t, "validation error", resBody["message"])
	})
}

func TestAuthHandler_Login_Errors(t *testing.T) {
	v := validator.New()
	handler := auth.NewAuthHandler(&stubAuthUsecase{}, v)
	app := setupHandlerTestApp(handler)

	t.Run("Invalid Request Body", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/login", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

		var resBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&resBody)
		assert.Equal(t, "invalid request body", resBody["message"])
	})

	t.Run("Validation Error", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email": "invalid-email",
		})
		req, _ := http.NewRequest("POST", "/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

		var resBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&resBody)
		assert.Equal(t, "validation error", resBody["message"])
	})
}
