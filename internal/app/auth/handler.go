package auth

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"

	pkgerr "invise-backend/pkg/errors"
)

type AuthHandler struct {
	usecase   AuthUsecaseI
	validator *validator.Validate
}

func NewAuthHandler(usecase AuthUsecaseI, v *validator.Validate) *AuthHandler {
	return &AuthHandler{usecase: usecase, validator: v}
}

func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req RegisterRequest
	if err := c.Bind().JSON(&req); err != nil {
		return pkgerr.BadRequest("INVALID_REQUEST", "invalid request body")
	}
	if err := h.validator.Struct(req); err != nil {
		return pkgerr.BadRequest("VALIDATION_ERROR", err.Error())
	}

	res, err := h.usecase.Register(c.Context(), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    fiber.Map{"message": res.Message},
	})
}

func (h *AuthHandler) Verify(c fiber.Ctx) error {
	var req VerifyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return pkgerr.BadRequest("INVALID_REQUEST", "invalid request body")
	}
	if err := h.validator.Struct(req); err != nil {
		return pkgerr.BadRequest("VALIDATION_ERROR", err.Error())
	}

	res, err := h.usecase.Verify(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    fiber.Map{"message": res.Message},
	})
}

func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req LoginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return pkgerr.BadRequest("INVALID_REQUEST", "invalid request body")
	}
	if err := h.validator.Struct(req); err != nil {
		return pkgerr.BadRequest("VALIDATION_ERROR", err.Error())
	}

	res, err := h.usecase.Login(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    fiber.Map{"access_token": res.AccessToken},
	})
}
