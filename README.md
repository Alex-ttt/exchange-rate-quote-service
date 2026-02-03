# Currency Quotes Service

[![CI](https://github.com/yourusername/exchange-rate-quote-service/actions/workflows/ci.yml/badge.svg)](https://github.com/yourusername/exchange-rate-quote-service/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourusername/exchange-rate-quote-service)](https://goreportcard.com/report/github.com/yourusername/exchange-rate-quote-service)
[![codecov](https://codecov.io/gh/yourusername/exchange-rate-quote-service/branch/main/graph/badge.svg)](https://codecov.io/gh/yourusername/exchange-rate-quote-service)

Asynchronous currency exchange rate quote service built with Go, implementing background job processing with Asynq, Redis caching, and PostgreSQL storage.

## Features

- **Asynchronous quote updates**: Request currency pair updates without blocking
- **Background processing**: Worker processes fetch rates from external provider with retries
- **Caching**: Redis-backed read-through cache for latest quotes
- **Deduplication**: In-flight updates for the same pair return the same update ID
- **Health checks**: Liveness and readiness endpoints for orchestration
- **Observability**: Request ID tracking and structured JSON logs
- **API Documentation**: Auto-generated Swagger/OpenAPI spec

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.25.6+ (for local development)

### Run with Docker Compose

```bash
docker compose up --build
```

This starts:
- **app** (HTTP API + worker) on port 8080
- **postgres** (18.1-alpine) on port 5432
- **redis** (8.4.0-alpine) on port 6380

### Test the API

```bash
# Health check
curl http://localhost:8080/healthz

# Readiness check
curl http://localhost:8080/readyz

# Request quote update (async)
curl -X POST http://localhost:8080/quotes/update \
  -H 'Content-Type: application/json' \
  -d '{"pair":"EUR/MXN"}'
# Response: {"update_id":"<uuid>","status":"PENDING"}

# Poll for result
curl http://localhost:8080/quotes/<update_id>
# Response (when ready): {"update_id":"...","base":"EUR","quote":"MXN","status":"SUCCESS","price":"18.7543","updated_at":"2025-12-01T10:15:30Z"}

# Get latest quote (cached)
curl "http://localhost:8080/quotes/latest?base=EUR&quote=MXN"
# Response: {"base":"EUR","quote":"MXN","price":"18.7543","updated_at":"2025-12-01T10:15:30Z"}
```

### Swagger UI

Open http://localhost:8080/swagger/index.html in your browser for interactive API documentation.

**Note:** Swagger docs are automatically generated during Docker build. For local development, regenerate after API changes with `make swagger`.

## API Endpoints

### POST /quotes/update

Request an asynchronous quote update.

**Request:**
```json
{"pair": "EUR/MXN"}
```

**Responses:**
- `202 Accepted`: Returns `update_id` for tracking
- `400 Bad Request`: Invalid currency code format (must be `[A-Z]{3}/[A-Z]{3}`)
- `500 Internal Server Error`: Unexpected error

**Idempotency:** Multiple requests for the same pair while processing return the same `update_id`.

### GET /quotes/{update_id}

Get status and result of an update request.

**Responses:**
- `200 OK`: Returns status and (if ready) price and timestamp
- `400 Bad Request`: Invalid UUID format
- `404 Not Found`: Unknown update ID

**Status values:**
- `PENDING`: Queued for processing
- `RUNNING`: Being processed
- `SUCCESS`: Complete with price and updated_at
- `FAILED`: Error occurred (includes error message)

### GET /quotes/latest

Get the most recent successful quote for a currency pair.

**Query params:**
- `base` (required): 3-letter currency code
- `quote` (required): 3-letter currency code

**Responses:**
- `200 OK`: Returns latest quote
- `400 Bad Request`: Invalid currency codes
- `404 Not Found`: No successful quote available

**Note:** This endpoint does NOT trigger a new fetch; it only returns cached/stored data.

### GET /healthz

Liveness probe. Always returns `200 OK` with `"OK"` if the service is running.

### GET /readyz

Readiness probe. Checks database and Redis connectivity.

**Responses:**
- `200 OK`: All dependencies ready
- `503 Service Unavailable`: At least one dependency unavailable

## Development

### GoLand / IntelliJ IDEA Setup

The project includes pre-configured run configurations in the `.run/` directory:

#### Available Run Configurations:

1. **Run App** - Start the application locally
   - Pre-configured with environment variables
   - Requires Docker dependencies running (see below)

2. **Run Tests** - Execute all tests

3. **Run Tests with Race Detector** - Run tests with `-race` flag

4. **Docker Compose Up** - Start all services (app + db + redis)

5. **Docker Dependencies Only** - Start only db + redis (for local app development)

6. **Build Binary** - Build the application binary

7. **Generate Swagger Docs** - Generate OpenAPI documentation

8. **Run Linter** - Execute golangci-lint

9. **Run All CI Checks** - Run fmt, vet, lint, and tests with race detector

#### Usage:

1. Open the project in GoLand/IntelliJ IDEA
2. The run configurations will appear automatically in the Run/Debug dropdown
3. To run the app locally:
   - First, run **Docker Dependencies Only** to start db + redis
   - Then, run **Run App** to start the application
   - Access at http://localhost:8080

### Quick Start with Make

```bash
# Show all available commands
make help

# Run all CI checks locally (fmt, vet, lint, test with race detector)
make ci

# Run tests
make test

# Run tests with race detector
make test-race

# Run tests with coverage report
make test-cover

# Build the binary
make build

# Start docker compose services
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

### Run Tests

```bash
# Basic tests
go test ./...

# With race detector
go test -race ./...

# With coverage
go test -coverprofile=coverage.out ./...
```

### Run Locally (without Docker)

```bash
# Start dependencies
docker compose up db redis

# Set environment variables (see .env.example)
export QUOTESVC_DATABASE_HOST=localhost
export QUOTESVC_REDIS_ADDR=localhost:6380

# Run application
go run ./cmd/app
```

### Generate Swagger Docs

```bash
# Install swag tool
go install github.com/swaggo/swag/cmd/swag@latest

# Generate docs
swag init -g cmd/app/main.go -o internal/api/docs

# Or use Make
make swagger
```

### Linting

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run --timeout=5m

# Or use Make
make lint
```

## Configuration

Configuration is loaded from:
1. `internal/config/config.yaml` (defaults)
2. Environment variables (overrides, prefix: `QUOTESVC_`)

See `.env.example` for all available variables.

### Key Configuration

- **HTTP_ADDR**: Server port (default: 8080)
- **POSTGRES_DSN**: Database connection string
- **REDIS_ADDR**: Redis address (default: redis:6380)
- **RATES_PROVIDER_BASE_URL**: External rates API URL
- **WORKER_CONCURRENCY**: Number of concurrent background jobs

## Architecture

### Components

- **HTTP API** (`internal/api`): Request handling and routing (Chi)
- **Service** (`internal/service`): Business logic
- **Repository** (`internal/repository`): Database access (PostgreSQL)
- **Worker** (`internal/worker`): Background job processing (Asynq)
- **Provider** (`internal/provider`): External rates API client
- **Cache**: Redis for latest quote caching

### Data Flow

1. **POST /quotes/update**: Validates pair, creates DB record (status=PENDING), enqueues job
2. **Worker**: Picks job, sets status=RUNNING, fetches rate from provider, updates DB (SUCCESS/FAILED) and cache
3. **GET /quotes/{update_id}**: Returns DB record by ID
4. **GET /quotes/latest**: Read-through Redis cache → DB → cache update

### State Machine

```
PENDING → RUNNING → SUCCESS
                 → FAILED (after retries)
```

### Deduplication

Partial unique index on `(base, quote) WHERE status IN ('PENDING','RUNNING')` ensures only one in-flight update per pair.

## Database Schema

**Table: quotes**

| Column       | Type         | Description                      |
|--------------|--------------|----------------------------------|
| id           | UUID         | Primary key                      |
| base         | CHAR(3)      | Base currency                    |
| quote        | CHAR(3)      | Quote currency                   |
| price        | NUMERIC(18,6)| Exchange rate (nullable)         |
| status       | ENUM         | PENDING/RUNNING/SUCCESS/FAILED   |
| error        | TEXT         | Error message (nullable)         |
| requested_at | TIMESTAMPTZ  | Request timestamp                |
| updated_at   | TIMESTAMPTZ  | Completion timestamp (nullable)  |

**Indexes:**
- `idx_quotes_pair_time`: `(base, quote, updated_at DESC)` for latest queries
- `uniq_quotes_pair_pending`: Unique `(base, quote)` where status in ('PENDING','RUNNING')

## Observability

### Request ID

- Header: `X-Request-Id`
- If not provided, a UUID is generated
- Returned in response header
- Included in all logs for request correlation

### Structured Logging

JSON logs with fields:
- `timestamp`, `level`, `msg`
- `request_id`, `method`, `path`, `status`, `duration_ms` (HTTP requests)
- `update_id`, `base`, `quote` (worker jobs)

## Testing

Test coverage includes:
- **API handlers**: Status codes, error shapes, validation
- **Middleware**: Request ID generation and propagation
- **Service**: Currency code validation, UUID validation
- **Worker processing**: Success and failure paths

### Test Commands

```bash
# Run all tests
make test

# Run tests with race detector (detects data races)
make test-race

# Generate coverage report (creates coverage.html)
make test-cover

# Or use go commands directly
go test ./...                              # Basic tests
go test -race ./...                        # Race detection
go test -coverprofile=coverage.out ./...  # Coverage
```

## CI/CD

The project includes comprehensive CI/CD pipeline with GitHub Actions (`.github/workflows/ci.yml`):

### CI Jobs

1. **Test**: Runs tests, race detector, and generates coverage
2. **Lint**: Runs golangci-lint for code quality checks
3. **Build**: Builds the binary and verifies it
4. **Docker**: Builds Docker image with caching
5. **Docker Compose**: Full integration test with all services
6. **Security**: Runs gosec security scanner

### Running CI Checks Locally

```bash
# Run all CI checks
make ci

# Or run individual checks
make fmt        # Format code
make vet        # Run go vet
make lint       # Run golangci-lint
make test-race  # Run tests with race detector
```

### Race Detector

The race detector is enabled in CI to catch data race conditions. It's also available locally:

```bash
go test -race ./...
# or
make test-race
```

**Note**: Race detector has a performance overhead (5-10x slower) and memory usage increase, so it's run separately from regular tests.

## Production Considerations

- **Secrets**: Use environment variables (never hard-code)
- **Security**: Container runs as non-root user
- **Timeouts**: External API calls timeout after 5s
- **Retries**: Asynq handles retries with exponential backoff
- **Graceful shutdown**: SIGINT/SIGTERM handled cleanly
- **Scalability**: Horizontal scaling supported (stateless + Redis coordination)

## License

See SPEC.md for complete requirements and acceptance criteria.
