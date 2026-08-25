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
	"invise-backend/internal/app/stocks"
	"invise-backend/internal/bootstrap/config"
	"invise-backend/internal/middleware"
	"invise-backend/pkg/ai"
	"invise-backend/pkg/response"
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

	// Shared services
	jwtSvc := pkgjwt.New(s.cfg.JWT.Secret, s.cfg.JWT.ExpiryMinutes)
	passSvc := pkgpassword.New()
	ulidSvc := pkgulid.New()
	mailer := pkgmail.New(s.cfg.SMTP.Host, s.cfg.SMTP.Port, s.cfg.SMTP.Username, s.cfg.SMTP.Password, s.cfg.SMTP.From)
	aiClient := ai.NewClient(s.cfg.AI.URL, s.cfg.AI.TimeoutSeconds)

	// Auth
	userRepo := auth.NewUserRepository(s.db)
	authUsecase := auth.NewAuthUsecase(userRepo, jwtSvc, passSvc, ulidSvc, mailer, s.rdb, s.cfg.OTP)
	authHandler := auth.NewAuthHandler(authUsecase, validator.New())

	authGroup := api.Group("/auth")
	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/verify", authHandler.Verify)
	authGroup.Post("/login", authHandler.Login)

	// Stocks & Market (Protected)
	authMiddleware := middleware.RequiredRoles(jwtSvc)
	stockRepo := stocks.NewStockRepository(s.db)
	stockUsecase := stocks.NewStockUsecase(stockRepo, aiClient, ulidSvc)
	stockHandler := stocks.NewStockHandler(stockUsecase)

	stocksGroup := api.Group("/stocks", authMiddleware)
	stocksGroup.Post("/import", stockHandler.Import)
	stocksGroup.Get("/", stockHandler.ListStocks)
	stocksGroup.Get("/items/:items_id", stockHandler.GetItemDetail)
	stocksGroup.Get("/items/:items_id/diagnose", stockHandler.GetItemDiagnose)
	stocksGroup.Get("/:stock_id/projection", stockHandler.GetStockProjection)
	stocksGroup.Get("/:stock_id", stockHandler.GetStockItems)


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

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
		message = fiberErr.Message
	}

	var appErr *pkgerr.AppError
	if errors.As(err, &appErr) {
		code = appErr.StatusCode
		message = appErr.Message
	}

	return c.Status(code).JSON(dto.Response[any]{
		Message: message,
	})
}

