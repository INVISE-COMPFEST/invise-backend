# Invise Backend

Invise Backend is a high-performance inventory analytics and deadstock diagnosis REST API built in Go with Fiber v3. It integrates with the Temporal Fusion Transformer (TFT) AI forecasting service ([`invise-ai-`](https://github.com/INVISE-COMPFEST/invise-ai-)) to predict demand, classify deadstock items, and generate actionable inventory recommendations.

---

## Features

- **Authentication & Security**: Email OTP verification, Argon2/Bcrypt password hashing, and JWT bearer token authentication.
- **AI-Powered Demand Forecasting**: Dispatches historical monthly sales time-series to the AI service for next-month predictions and feature importance analysis.
- **Deadstock Diagnosis**: Automatically classifies inventory items into `HEALTHY`, `SLOW_MOVING`, or `DEADSTOCK` with recommended actions (`RESTOCK`, `DISCOUNT`, `LIQUIDATE`, `HOLD`).
- **Time-Series Projections**: Historical sales curve tracking and forecasted trajectory points for SKU-level visualizations.
- **Multi-Format CSV Ingestion**: Supports flexible CSV headers conforming to evaluation datasets (`item_modal.csv`, `item_inventory.csv`, and monthly sales samples).

---

## Tech Stack

| Component | Technology |
|---|---|
| **Language** | Go 1.22+ |
| **HTTP Framework** | [Fiber v3](https://github.com/gofiber/fiber) |
| **Database** | PostgreSQL 16+ via [GORM](https://gorm.io) |
| **Migrations** | [Goose v3](https://github.com/pressly/goose) |
| **Cache & OTP Store** | Valkey / Redis via [go-redis v9](https://github.com/redis/go-redis) |
| **Object Storage** | MinIO (S3-compatible) |
| **AI Integration** | REST client to `invise-ai-` (PyTorch Forecasting TFT) |
| **Containerization** | Docker / Podman Compose & Multi-stage `Containerfile` |

---

## Prerequisites

Ensure you have the following installed on your machine:

- **Go**: `1.22` or higher ([Download Go](https://go.dev/dl/))
- **Docker** & **Docker Compose** (or **Podman** / **Podman Compose**)
- **Git**

*(Optional for local development without containers)*:
- **PostgreSQL**: `15+`
- **Valkey** / **Redis**: `7+`
- **Air**: For live reload (`go install github.com/air-verse/air@latest`)
- **golangci-lint**: For linting (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)

---

## Quick Start (Docker Compose)

The easiest way to run the entire backend infrastructure (API server, PostgreSQL, Valkey, and MinIO) is using Docker Compose.

### 1. Clone the repository

```bash
git clone https://github.com/INVISE-COMPFEST/invise-backend.git
cd invise-backend
```

### 2. Configure environment variables

Copy the example environment file and customize secrets as needed:

```bash
make setup
```

### 3. Start all services

```bash
make up
# or: docker compose up -d
```

This starts:
- **API Server**: http://localhost:8080
- **PostgreSQL**: `localhost:5432`
- **Valkey (Redis)**: `localhost:6379`
- **MinIO Storage**: http://localhost:9000 (Console: http://localhost:9001)

### 4. Run database migrations

```bash
make db-migrate
# or: go run ./cmd/migrate up
```

### 5. Verify service health

```bash
make health
# or: curl http://localhost:8080/health
```

---

## Local Development (Without Containers)

If you prefer to run the Go application directly on your host machine:

### 1. Setup development files and dependencies

```bash
make setup
```

### 2. Start backing services

Start only the database, cache, and storage containers:

```bash
docker compose up -d postgres valkey minio
```

### 3. Run migrations

```bash
make db-migrate
```

### 4. Run the application

```bash
# Standard run
make run

# Or with hot-reload (Air)
make dev
```

---

## Configuration Reference (`.env`)

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | Environment mode (`development`, `production`, `test`) |
| `APP_PORT` | `8080` | Port on which the HTTP server listens |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | PostgreSQL username |
| `DB_PASSWORD` | `postgres` | PostgreSQL password |
| `DB_NAME` | `backend` | PostgreSQL database name |
| `DB_SSL_MODE` | `disable` | PostgreSQL SSL mode (`disable`, `require`) |
| `DB_PATH` | `public` | PostgreSQL schema search path |
| `JWT_SECRET` | - | Secret key used for signing JWT tokens (min 32 chars) |
| `JWT_EXPIRY_MINUTES` | `60` | JWT access token validity in minutes |
| `OTP_TTL_MINUTES` | `5` | Email verification OTP time-to-live in minutes |
| `SMTP_HOST` | `smtp.gmail.com` | SMTP relay server hostname |
| `SMTP_PORT` | `587` | SMTP relay port |
| `SMTP_USERNAME` | - | SMTP authentication username |
| `SMTP_PASSWORD` | - | SMTP authentication app password |
| `SMTP_FROM` | - | Sender email address for outgoing OTPs |
| `VALKEY_HOST` | `localhost` | Valkey / Redis host |
| `VALKEY_PORT` | `6379` | Valkey / Redis port |
| `VALKEY_PASSWORD` | - | Valkey / Redis password (leave empty if none) |
| `VALKEY_DB` | `0` | Valkey database index |
| `MINIO_ENDPOINT` | `localhost:9000` | S3 / MinIO endpoint |
| `MINIO_ACCESS_KEY` | `minioadmin` | MinIO root access key |
| `MINIO_SECRET_KEY` | `minioadmin123` | MinIO root secret key |
| `MINIO_BUCKET` | `backend` | Target S3 bucket name |
| `AI_SERVICE_URL` | `http://localhost:5000` | URL of the `invise-ai-` demand forecasting service |
| `AI_SERVICE_TIMEOUT_SECONDS` | `60` | HTTP request timeout for AI predictions |

---

## Database Migrations

Database migrations are managed using [Goose](https://github.com/pressly/goose) with SQL files located in `db/migrations/`.

```bash
# Apply all pending migrations
make db-migrate

# Rollback the last migration
make db-rollback

# Check migration status
make db-status

# Check current migration version
make db-version

# Create a new migration file
make db-create name=add_column_to_items

# Reset all migrations (drops all tables)
make db-reset
```

---

## API Endpoints

Full interactive OpenAPI documentation is available in [`docs/api.openapi.json`](docs/api.openapi.json).

### 1. Authentication

| Method | Endpoint | Description | Auth |
|---|---|---|---|
| `POST` | `/api/v1/auth/register` | Register new account and send OTP code to email | Public |
| `POST` | `/api/v1/auth/verify` | Verify email with 6-digit OTP code | Public |
| `POST` | `/api/v1/auth/login` | Authenticate user and receive JWT bearer token | Public |

### 2. Stock Management & Ingestion

| Method | Endpoint | Description | Auth |
|---|---|---|---|
| `POST` | `/api/v1/stocks/import` | Upload 3 CSV files, trigger AI forecast, and persist batch | Bearer |
| `GET` | `/api/v1/stocks` | List stock batch IDs for the authenticated user | Bearer |
| `GET` | `/api/v1/stocks/:stock_id` | List items belonging to a stock batch | Bearer |
| `GET` | `/api/v1/stocks/items/:items_id` | Get item details (pricing, age, sales days) | Bearer |
| `GET` | `/api/v1/stocks/items/:items_id/diagnose` | Get deadstock diagnostic reasons & cost metrics | Bearer |
| `GET` | `/api/v1/stocks/:stock_id/projection` | Get time-series historical & predicted curve points | Bearer |

### 3. System Health

| Method | Endpoint | Description | Auth |
|---|---|---|---|
| `GET` | `/health` | Service health check | Public |

---

## CSV Data Import Format

The `POST /api/v1/stocks/import` endpoint expects a `multipart/form-data` request containing **three CSV files**:

### 1. Monthly Sales Data (`monthly_sales_data` / `sales`)
Historical monthly sales records per product series.
- **Required / Accepted Columns**:
  - `date_month` or `month` (e.g. `2016-05`)
  - `id` or `series_id` (e.g. `FOODS_1_035_CA_1_evaluation`)
  - `item_id` or `sku` (e.g. `FOODS_1_035`)
  - `store_id` or `store` (e.g. `CA_1`)
  - `monthly_sales` or `sales` (e.g. `9`)
  - *(Optional)*: `dept_id`, `cat_id`, `state_id`

### 2. Unit Cost / Modal Data (`unit_cost_data` / `item_modal` / `modal`)
Product cost / capital investment data.
- **Required / Accepted Columns**:
  - `id` or `item_id` or `sku` (e.g. `FOODS_1_035_CA_1_evaluation`)
  - `modal` or `unit_cost` or `cost` or `price` (e.g. `71.29`)
  - *(Optional)*: `month`, `product_name`, `store`

### 3. Stock Level / Inventory Data (`stock_level_data` / `item_inventory` / `inventory`)
Current inventory on hand per item.
- **Required / Accepted Columns**:
  - `id` or `item_id` or `sku` (e.g. `FOODS_1_035_CA_1_evaluation`)
  - `inventory` or `quantity` or `stock_level` (e.g. `65`)
  - *(Optional)*: `month`, `product_name`, `store`

> [!NOTE]
> For multi-month files (such as `item_modal.csv` and `item_inventory.csv`), the importer automatically tracks and resolves the latest month record for each item.

---

## Testing & Quality Control

Run unit and integration test suites:

```bash
# Run all unit tests with race detection
make test

# Run tests with coverage output
make test-cover

# Format source code
make fmt

# Run linter
make lint

# Compile production binaries
make build
```

---

## Project Structure

```
.
├── cmd/
│   ├── api/             # Main application entry point
│   └── migrate/         # Goose migration CLI runner
├── db/
│   └── migrations/      # SQL migration scripts
├── docs/
│   ├── adr/             # Architecture Decision Records
│   └── api.openapi.json # OpenAPI 3.0 specification
├── internal/
│   ├── app/
│   │   ├── auth/        # Authentication module (register, verify, login)
│   │   └── stocks/      # Stock ingestion, diagnosis, and projections
│   ├── bootstrap/
│   │   ├── config/      # Environment configuration loader
│   │   ├── server/      # Fiber HTTP routing and middleware setup
│   │   └── valkey/      # Valkey / Redis client initialization
│   └── middleware/      # JWT authentication and authorization
├── pkg/
│   ├── ai/              # AI service HTTP client
│   ├── errors/          # Structured application errors
│   ├── jwt/             # JWT token generator and parser
│   ├── mail/            # SMTP mailer for OTP delivery
│   ├── password/        # Password hashing utilities
│   ├── response/        # Standardized API response wrappers
│   └── ulid/            # ULID generator
├── compose.yml          # Docker Compose multi-service definition
├── Containerfile        # Multi-stage container build definition
├── Makefile             # Development automation tasks
└── README.md
```

---

## Useful `make` Commands

| Target | Description |
|---|---|
| `make help` | Display all available make targets |
| `make setup` | Initialize development environment and dependencies |
| `make dev` | Start server with live reload via Air |
| `make up` | Start all docker compose containers in background |
| `make down` | Stop all docker compose containers |
| `make logs` | Tail logs of the application container |
| `make db-shell` | Open interactive `psql` session inside the database container |
| `make db-backup` | Dump database into `backups/` directory |
| `make db-restore` | Restore database dump (`make db-restore FILE=backups/backup.sql`) |
| `make clean` | Remove build binaries and log files |
