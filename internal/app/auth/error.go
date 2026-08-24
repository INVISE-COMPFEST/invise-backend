package auth

import pkgerr "invise-backend/pkg/errors"

var (
	ErrInvalidCredentials = pkgerr.Unauthorized("INVALID_CREDENTIALS", "invalid email or password")
	ErrEmailAlreadyExists = pkgerr.Conflict("EMAIL_ALREADY_EXISTS", "email is already registered")
	ErrUserNotFound       = pkgerr.NotFound("USER_NOT_FOUND", "user not found")
	ErrInvalidOTP         = pkgerr.BadRequest("INVALID_OTP", "invalid or expired OTP")
	ErrAccountNotVerified = pkgerr.Unauthorized("ACCOUNT_NOT_VERIFIED", "account not verified")
)
