# BCC Backend — Go Project Reference

> Source: `github.com/bccfilkom/backend` (Go 1.24)
> Generated: 2026-08-24

---

## 1. Project Layout

```
bcc-backend/
├── cmd/
│   ├── app/main.go              # Application entrypoint (Fiber HTTP server)
│   └── migrate/main.go          # Standalone goose migration CLI
├── db/
│   ├── migrations/              # SQL migration files (goose)
│   ├── postgre/postgre.go       # GORM + pgx pool helpers
│   └── seeders/                 # Seed scripts (one file per entity)
├── internal/
│   ├── api/                     # Domain-organized API packages (14 domains)
│   │   ├── auth/
│   │   ├── challenge/
│   │   ├── champion/
│   │   ├── department/
│   │   ├── division/
│   │   ├── document/
│   │   ├── events/
│   │   ├── member/
│   │   ├── project/
│   │   ├── recruitment/
│   │   ├── showcase/
│   │   ├── testimonial/
│   │   ├── user/
│   │   └── workshop/
│   ├── bootstrap/
│   │   ├── config/config.go     # Env-based configuration
│   │   └── server/server.go     # Fiber server + route registration + DI wiring
│   ├── common/                  # Shared domain value objects
│   ├── middleware/               # Auth, CORS, error handler, rate limiter, request ID, helmet
│   ├── scheduler/               # Cron-based presence scheduler
│   └── utils/                   # errors, file validation, idgen, logger
├── pkg/                         # Reusable library packages
│   ├── bcrypt/
│   ├── jwt/
│   ├── objectstorage/           # MinIO client wrapper
│   ├── pagination/
│   ├── ulid/
│   └── validator/
├── Makefile                     # Build, test, docker, db commands
├── docker-compose.yml
├── Dockerfile
├── go.mod / go.sum
└── README.md
```

**Key files for orientation:**
- `cmd/app/main.go:18-62` — bootstrap sequence (config → logger → DB → MinIO → JWT → server)
- `internal/bootstrap/server/server.go:65-115` — `Server` struct, `New()` constructor, middleware stack
- `internal/bootstrap/server/server.go:170-412` — `registerRoutes()` — all route registration and DI wiring
- `internal/bootstrap/config/config.go:26-34` — `Config` struct with all sub-configs

---

## 2. Architecture: Clean Architecture (Handler → Usecase → Repository)

Each API domain follows the same internal structure:

```
internal/api/<domain>/
├── domain/          # Entity structs (DB-mapped), value objects, formatting methods
├── dto.go           # Request/response DTOs, pagination params, constants
├── error.go         # Pre-defined domain errors (AppError instances)
├── handler/
│   └── rest/        # Fiber HTTP handlers
│       ├── rest.go  # Handler struct + New() constructor
│       └── <domain>.go  # Handler methods
├── repository/
│   ├── repository.go   # Interface definition + concrete impl + New()
│   ├── <domain>.go     # Repository method implementations
│   └── mock/           # Generated mocks (mockgen)
└── usecase/
    ├── usecase.go      # Interface definition + concrete impl + New()
    ├── <domain>.go     # Usecase method implementations
    ├── helper.go       # Optional helper functions
    └── test/           # Generated mocks / tests
```

**Observed in:**
- `internal/api/workshop/` — full domain with registrants, feedbacks, certificates
- `internal/api/auth/` — simpler domain (login + session)
- `internal/api/document/` — uses pgx directly instead of GORM

### Dependency Flow

```
Handler → Usecase → Repository → Database (GORM or pgx)
                ↘ MinIO (object storage)
```

### DI Wiring Pattern

All dependencies are constructed in `server.registerRoutes()` (`server.go:170-412`):

```go
// 1. Repository (receives *gorm.DB + logger)
workshopRepository := workshopRepo.New(s.db.DB, s.logger)

// 2. Usecase (receives repository interface + dependencies)
workshopUseCase := workshopUsecase.New(workshopRepository, s.minioClient, s.logger, ulid, sched)

// 3. Handler (receives usecase interface + validator + logger)
workshopHandler := workshopRest.New(workshopUseCase, s.validator, s.logger)
```

**No DI framework** — pure constructor injection. Each layer depends only on the interface above it.

---

## 3. Go Patterns

### 3.1 Interface Naming Convention

Interfaces use the `I` suffix: `WorkshopRepositoryI`, `WorkshopUseCaseI`, `JwtI`, `BcryptI`, `UlidI`, `MinioI`.

```go
// internal/api/workshop/repository/repository.go:14
type WorkshopRepositoryI interface { ... }

// pkg/jwt/jwt.go:11
type JwtI interface { ... }

// pkg/bcrypt/bcrypt.go:5
type BcryptI interface { ... }
```

### 3.2 Constructor Pattern

Every package exposes a `New()` function that returns the interface type:

```go
// repository/repository.go:48
func New(db *gorm.DB, l *logger.Logger) WorkshopRepositoryI {
    return &WorkshopRepository{db: db, l: l}
}

// usecase/usecase.go:42
func New(cr workshopRepository.WorkshopRepositoryI, os objectstorage.MinioI, ...) WorkshopUseCaseI {
    return &WorkshopUsecase{cr: cr, os: os, ...}
}

// handler/rest/rest.go:16
func New(cu workshopUseCase.WorkshopUseCaseI, v *validator.Validate, l *logger.Logger) *WorkshopHandler {
    return &WorkshopHandler{cu: cu, v: v, l: l}
}
```

### 3.3 Error Handling

**Custom AppError type** (`internal/utils/errors/errors.go:9-15`):

```go
type AppError struct {
    StatusCode int         `json:"-"`
    Code       string      `json:"code"`
    Message    string      `json:"message"`
    Details    interface{} `json:"details,omitempty"`
    Err        error       `json:"-"`
}
```

- Implements `error` and `Unwrap()` for `errors.Is/As` compatibility.
- Convenience constructors: `New()`, `Wrap()`, `BadRequest()`, `Unauthorized()`, `Forbidden()`, `NotFound()`, `InternalServerError()`.
- Domain errors are pre-declared as package-level vars using `errors.New(statusCode, code, message, nil)`.

**Error pattern in domain files** (`internal/api/workshop/error.go`):

```go
var (
    ErrWorkshopNotFound     = _errors.New(404, "WORKSHOP_NOT_FOUND", "workshop not found", nil)
    ErrAlreadyRegistered    = _errors.New(409, "ALREADY_REGISTERED", "...", nil)
    ErrInvalidPresenceCode  = _errors.New(400, "INVALID_PRESENCE_CODE", "...", nil)
)
```

**Error handling in handlers** — errors bubble up from usecase → handler → Fiber error handler:

```go
// handler/workshop.go:14-31
func (h *WorkshopHandler) Get(c *fiber.Ctx) error {
    ctx := c.UserContext()
    res, err := h.cu.Get(ctx, workshop.ParamWorkshop{ID: workshopID})
    if err != nil {
        return err  // bubbles to ErrorHandler middleware
    }
    return c.JSON(fiber.Map{"payload": res, "success": true})
}
```

**Global error handler** (`internal/middleware/error_handler.go:13-71`) — maps `AppError`, `fiber.Error`, and `validator.ValidationErrors` to JSON responses with appropriate status codes. In production, generic errors are masked.

### 3.4 Logging

Uses **logrus** wrapped in a custom `Logger` struct (`internal/utils/logger/logger.go`).

- Structured fields via `WithFields(map[string]any{...})`.
- JSON or text format based on config.
- File rotation via **lumberjack**.
- Request-scoped logging: request ID and user ID injected via `WithContext()`.
- Logging pattern is pervasive — every error path logs context before returning.

```go
r.l.WithFields(map[string]any{
    "error": err.Error(),
}).Error("failed to get all workshop")
```

### 3.5 Validation

Uses **go-playground/validator/v10**. Validation tags are on DTO struct fields:

```go
// internal/api/workshop/dto.go:32-45
type CreateWorkshop struct {
    Title       string                `form:"title" validate:"required"`
    LocationType string               `form:"location_type" validate:"required,oneof=online offline"`
    // ...
}
```

Validation is invoked in handlers via `h.v.Struct(req)`. Errors are translated by `internal/utils/errors/validation.go`.

### 3.6 Configuration

Environment-based via `godotenv` (`internal/bootstrap/config/config.go:71-163`). Every config value has a fallback default. Structured into sub-configs: `ServerConfig`, `DatabaseConfig`, `JWTConfig`, `MinIOConfig`, `SeederConfig`, `LogConfig`.

---

## 4. Database Patterns

### 4.1 GORM (primary ORM)

Used by most repositories. `*gorm.DB` is passed through constructors.

**Query building** (`internal/api/workshop/repository/workshop.go:14-51`):

```go
query := r.db.WithContext(ctx).Model(&domain.Workshop{})
if len(param.Topics) > 0 {
    query = query.Where("topic IN ?", param.Topics)
}
err := query.Count(&total).Error
```

**Error translation** — `gorm.ErrRecordNotFound` is mapped to domain errors:

```go
if errors.Is(err, gorm.ErrRecordNotFound) {
    return ws, workshop.ErrWorkshopNotFound
}
```

**Raw SQL** — used for complex queries with JOINs (`workshop.go:353-386`):

```go
q := r.db.WithContext(ctx).
    Table("workshop_feedbacks wf").
    Select(`wf.id, wf.workshop_registrant_id, wr.name, ...`).
    Joins("JOIN workshop_registrants wr ON wr.id = wf.workshop_registrant_id").
    Where("wr.workshop_id = ?", param.WorkshopID)
```

### 4.2 pgx (direct PostgreSQL)

Used by document/project repositories (`internal/api/document/repository/document.go`). Uses `pgxpool.Pool` with named args:

```go
query := `SELECT ... FROM "documents" WHERE "project_id" = @project_id`
args := pgx.NamedArgs{"project_id": projectID}
rows, err := dr.db.Query(ctx, query, args)
```

### 4.3 Database Connection

Dual setup in `db/postgre/postgre.go`:
- `NewPostgresConnection()` — returns `*Database` (GORM wrapper) for most services
- `NewPgxPool()` — returns `*pgxpool.Pool` for document/project services

### 4.4 Migrations

**Goose** SQL migrations in `db/migrations/`. Standalone CLI in `cmd/migrate/main.go`.

### 4.5 ID Generation

**ULIDs** (`pkg/ulid/ulid.go`) for primary keys. Interface-wrapped for testability.

**Custom ID generator** (`internal/utils/idgen/idgen.go`) for random codes (presence codes, referral codes).

---

## 5. Middleware Stack

Applied in order in `server.New()` (`server.go:92-100`):

1. **Recover** — `fiber/middleware/recover` (panic recovery)
2. **CORS** — `DevelopmentCORS` or `ProductionCORS` based on `APP_ENV`
3. **RequestID** — generates/propagates `X-Request-ID` UUID
4. **RequestLogger** — structured request/response logging with duration, status, user ID
5. **Helmet** — security headers (XSS, HSTS, CSP, etc.)

**Per-route middleware:**
- `RequiredRoles()` — JWT auth with role-based access
- `PublicEndpointRateLimiter` — 200 req/min for public endpoints
- `StrictRateLimiter` — 10 req/min for admin endpoints
- `timeout.NewWithContext()` — per-route request timeouts

**Auth flow** (`internal/middleware/auth.go:73-117`):
1. Extract Bearer token from `Authorization` header
2. Validate JWT via `jwtManager.Validate()`
3. Extract `user_id` and `role` into `c.Locals()`
4. Check required roles

---

## 6. Object Storage (MinIO)

Wrapper in `pkg/objectstorage/minio.go`. Interface `MinioI`:

```go
type MinioI interface {
    UploadFile(ctx, reader, objectName, size, contentType) (string, error)
    DeleteFile(ctx, objectName) error
    GetFile(ctx, objectName) (*url.URL, error)
    GetFileWithExpiry(ctx, objectName, expiry) (*url.URL, error)
}
```

- Auto-creates bucket with public read policy on startup.
- Generates presigned URLs (15min default) or converts to public URLs if configured.
- `RawClient()` exposes underlying `*minio.Client` for direct use (document/project repos).

---

## 7. Testing Patterns

### 7.1 Mock Generation

Uses **mockgen** (both `github.com/golang/mock` and `go.uber.org/mock`):

```bash
mockgen -source=internal/api/workshop/repository/repository.go \
    -destination=internal/api/workshop/repository/mock/workshop_mock.go
```

Mocks live in `mock/` subdirectories alongside their interfaces.

### 7.2 Test Location

Tests co-located with usecases: `internal/api/workshop/usecase/test/workshop_test.go`.

### 7.3 Mock Libraries

Two mock libraries in use:
- `github.com/golang/mock` — used in some usecase tests
- `go.uber.org/mock` — used in repository mocks

---

## 8. Conventions Summary

| Aspect | Convention |
|--------|-----------|
| **Module path** | `github.com/bccfilkom/backend` |
| **Go version** | 1.24 |
| **HTTP framework** | Fiber v2 |
| **ORM** | GORM (primary), pgx (direct SQL) |
| **Config** | env vars via godotenv |
| **Logging** | logrus + lumberjack rotation |
| **ID generation** | ULIDs for PKs, custom random for codes |
| **Error codes** | HTTP status + machine-readable code string |
| **Validation** | go-playground/validator with struct tags |
| **Object storage** | MinIO |
| **Migrations** | Goose SQL files |
| **Cron** | robfig/cron for scheduled tasks |
| **Testing** | mockgen-generated mocks |
| **API prefix** | `/api/v1` |
| **JSON response shape** | `{"success": bool, "data": ..., "pagination": ..., "error": ...}` |
| **Interface suffix** | `I` (e.g., `WorkshopRepositoryI`) |
| **Error var prefix** | `Err` (e.g., `ErrWorkshopNotFound`) |
| **File naming** | snake_case for Go files |
| **Package naming** | lowercase, single-word |
| **Import grouping** | stdlib, external, internal (no explicit separation enforced) |
| **Context propagation** | `c.UserContext()` in handlers, `ctx` parameter everywhere |
| **Domain structs** | `db:"..."` tags for GORM, `json:"..."` for DTOs |
| **Table names** | `TableName()` methods on domain structs |
| **Formatting methods** | `Format<Name>Response()` on domain structs for response mapping |

---

## 9. Key Files Quick Reference

| File | Purpose |
|------|---------|
| `cmd/app/main.go` | App entrypoint — bootstrap sequence |
| `cmd/migrate/main.go` | Standalone migration CLI |
| `internal/bootstrap/config/config.go` | All configuration structs + env loading |
| `internal/bootstrap/server/server.go` | Fiber server, DI wiring, route registration |
| `internal/utils/errors/errors.go` | `AppError` type + constructors |
| `internal/middleware/error_handler.go` | Global Fiber error handler |
| `internal/middleware/auth.go` | JWT auth + role-based middleware |
| `internal/middleware/limiter.go` | Rate limiter configs |
| `db/postgre/postgre.go` | GORM + pgx connection helpers |
| `pkg/jwt/jwt.go` | JWT generation + validation |
| `pkg/objectstorage/minio.go` | MinIO wrapper |
| `pkg/pagination/pagination.go` | Pagination metadata + response |
| `pkg/validator/go_validator.go` | Validator factory |
| `Makefile` | Build, test, docker, db commands |
