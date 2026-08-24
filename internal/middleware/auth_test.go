package middleware

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"

	pkgerr "invise-backend/pkg/errors"
	"invise-backend/pkg/jwt"
)

func testErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "internal server error"
	machineCode := "INTERNAL_ERROR"

	var appErr *pkgerr.AppError
	if errors.As(err, &appErr) {
		code = appErr.StatusCode
		message = appErr.Message
		machineCode = appErr.Code
	}

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"error":   fiber.Map{"code": machineCode, "message": message},
	})
}

func okHandler(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

func setupTestApp(middlewareHandler fiber.Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: testErrorHandler})
	app.Get("/protected", middlewareHandler, okHandler)
	return app
}

func TestRequiredRoles_NoHeader(t *testing.T) {
	jwtSvc := jwt.New("test-secret", 60)
	app := setupTestApp(RequiredRoles(jwtSvc))

	req, _ := http.NewRequest("GET", "/protected", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestRequiredRoles_InvalidFormat(t *testing.T) {
	jwtSvc := jwt.New("test-secret", 60)
	app := setupTestApp(RequiredRoles(jwtSvc))

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Token abc123")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestRequiredRoles_InvalidToken(t *testing.T) {
	jwtSvc := jwt.New("test-secret", 60)
	app := setupTestApp(RequiredRoles(jwtSvc))

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestRequiredRoles_ValidToken(t *testing.T) {
	jwtSvc := jwt.New("test-secret", 60)
	app := setupTestApp(RequiredRoles(jwtSvc))

	token, err := jwtSvc.Generate("user-123", "test@example.com")
	assert.NoError(t, err)

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRequiredRoles_NoRolesRestriction(t *testing.T) {
	jwtSvc := jwt.New("test-secret", 60)
	app := setupTestApp(RequiredRoles(jwtSvc))

	token, err := jwtSvc.Generate("user-123", "test@example.com")
	assert.NoError(t, err)

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRequiredRoles_WithRoleRejects(t *testing.T) {
	jwtSvc := jwt.New("test-secret", 60)
	app := setupTestApp(RequiredRoles(jwtSvc, "admin"))

	token, err := jwtSvc.Generate("user-123", "test@example.com")
	assert.NoError(t, err)

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}