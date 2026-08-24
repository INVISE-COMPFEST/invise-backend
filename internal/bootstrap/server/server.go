package server

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"invise-backend/internal/app/auth"
	"invise-backend/internal/bootstrap/config"
	"invise-backend/internal/middleware"
	pkgerr "invise-backend/pkg/errors"
	pkgjwt "invise-backend/pkg/jwt"
	pkgmail "invise-backend/pkg/mail"
	pkgpassword "invise-backend/pkg/password"
	pkgulid "invise-backend/pkg/ulid"
)

type Server struct {
	app *fiber.App
	db  *gorm.DB
	rdb *redis.Client
	cfg config.Config
}

func New(cfg config.Config, db *gorm.DB, rdb *redis.Client) *Server {
	app := fiber.New(fiber.Config{
		AppName:      "Invise API",
		ErrorHandler: errorHandler,
	})

	app.Use(recover.New())
	app.Use(cors.New())
	app.Use(requestid.New())

	s := &Server{app: app, db: db, rdb: rdb, cfg: cfg}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	api := s.app.Group("/api/v1")

	// Auth
	jwtSvc := pkgjwt.New(s.cfg.JWT.Secret, s.cfg.JWT.ExpiryMinutes)
	passSvc := pkgpassword.New()
	ulidSvc := pkgulid.New()
	mailer := pkgmail.New(s.cfg.SMTP.Host, s.cfg.SMTP.Port, s.cfg.SMTP.Username, s.cfg.SMTP.Password, s.cfg.SMTP.From)
	userRepo := auth.NewUserRepository(s.db)
	authUsecase := auth.NewAuthUsecase(userRepo, jwtSvc, passSvc, ulidSvc, mailer, s.rdb, s.cfg.OTP)
	authHandler := auth.NewAuthHandler(authUsecase, validator.New())

	authGroup := api.Group("/auth")
	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/verify", authHandler.Verify)
	authGroup.Post("/login", authHandler.Login)

	// Protected routes (no role restriction — any authenticated user)
	_ = middleware.RequiredRoles(jwtSvc) // reserved for future protected routes

	// Health
	s.app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
}

func (s *Server) Listen(addr string) error {
	return s.app.Listen(addr)
}

func errorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "internal server error"
	_machineCode := "INTERNAL_ERROR"

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
		message = fiberErr.Message
	}

	var appErr *pkgerr.AppError
	if errors.As(err, &appErr) {
		code = appErr.StatusCode
		message = appErr.Message
		_machineCode = appErr.Code
	}

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":    _machineCode,
			"message": message,
		},
	})
}
