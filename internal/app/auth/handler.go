package auth

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"

	"invise-backend/pkg/response"
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
		return pkgerr.ErrInvalidRequest
	}
	if err := h.validator.Struct(req); err != nil {
		return pkgerr.ErrValidationError
	}

	if err := h.usecase.Register(c.Context(), req); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dto.Response[any]{
		Message: "registration successful, please check your email for verification code",
	})
}

func (h *AuthHandler) Verify(c fiber.Ctx) error {
	var req VerifyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return pkgerr.ErrInvalidRequest
	}
	if err := h.validator.Struct(req); err != nil {
		return pkgerr.ErrValidationError
	}

	if err := h.usecase.Verify(c.Context(), req); err != nil {
		return err
	}
	return c.JSON(dto.Response[any]{
		Message: "account verified successfully",
	})
}

func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req LoginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return pkgerr.ErrInvalidRequest
	}
	if err := h.validator.Struct(req); err != nil {
		return pkgerr.ErrValidationError
	}

	res, err := h.usecase.Login(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(dto.Response[*TokenResponse]{
		Message: "login successful",
		Data:    res,
	})
}
