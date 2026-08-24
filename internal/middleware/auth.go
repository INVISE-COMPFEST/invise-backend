package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	pkgerr "invise-backend/pkg/errors"
	"invise-backend/pkg/jwt"
)

func RequiredRoles(jwtSvc jwt.JwtI, roles ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return pkgerr.Unauthorized("MISSING_TOKEN", "authorization header is required")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return pkgerr.Unauthorized("INVALID_TOKEN_FORMAT", "authorization header must be: Bearer <token>")
		}

		claims, err := jwtSvc.Validate(parts[1])
		if err != nil {
			return pkgerr.Unauthorized("INVALID_TOKEN", "invalid or expired token")
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)

		if len(roles) > 0 {
			return pkgerr.Forbidden("INSUFFICIENT_ROLE", "role-based access not implemented")
		}

		return c.Next()
	}
}
