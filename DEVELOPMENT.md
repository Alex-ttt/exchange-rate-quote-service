# Development Guide

Quick reference for developers working on the Currency Quotes Service.

## Quick Start (GoLand)

### First Time Setup

1. **Clone and Open**
   ```bash
   git clone <repository-url>
   cd exchange-rate-quote-service
   ```
   Open in GoLand

2. **Install Dependencies**
   ```bash
   go mod download
   ```

3. **Install Development Tools**
   ```bash
   make install-tools
   # or manually:
   go install github.com/swaggo/swag/cmd/swag@latest
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   ```

### Daily Development Workflow

#### Option 1: Full Local Development (Recommended)

```
┌─────────────────────────────────────────────┐
│  Step 1: Start Dependencies                 │
│  Run Configuration: "Docker Dependencies    │
│  Only"                                       │
│  • Starts: Postgres + Redis                 │
│  • Ports: 5432, 6380                        │
└─────────────────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────┐
│  Step 2: Start Application                  │
│  Run Configuration: "Run App"               │
│  • Starts: Go application                   │
│  • Port: 8080                               │
│  • Debugger: Available ✓                    │
└─────────────────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────┐
│  Step 3: Access Application                 │
│  • API: http://localhost:8080               │
│  • Swagger: http://localhost:8080/swagger/  │
│  • Health: http://localhost:8080/healthz    │
└─────────────────────────────────────────────┘
```

**Advantages:**
- ✅ Full debugger support
- ✅ Fast code reload
- ✅ IDE integration
- ✅ Lower resource usage

#### Option 2: Full Docker

```
Run Configuration: "Docker Compose Up"
↓
Everything starts together (app + db + redis)
↓
Access at http://localhost:8080
```

**Use when:**
- Testing production-like environment
- Verifying Docker builds
- Quick demo/testing

## Run Configurations Cheat Sheet

| Configuration | When to Use | Prerequisites |
|--------------|-------------|---------------|
| **Run App** | Daily development | Docker Dependencies running |
| **Docker Dependencies Only** | Before local development | Docker running |
| **Docker Compose Up** | Full integration test | Docker running |
| **Run Tests** | After code changes | None |
| **Run Tests with Race Detector** | Before committing | None |
| **Run Integration Tests** | Integration tests (testcontainers) | Docker running |
| **Run All CI Checks** | Before push/PR | golangci-lint installed |
| **Build Binary** | Test build | None |
| **Generate Swagger Docs** | After API changes | swag installed |
| **Run Linter** | Code quality check | golangci-lint installed |

## Common Tasks

### Making API Changes

```
1. Modify handler in internal/api/handlers.go
2. Add/update Swagger annotations
3. Run: "Generate Swagger Docs"
4. Run: "Run Tests"
5. Test manually via Swagger UI or curl
```

### Adding New Endpoint

```
1. Add handler function with Swagger annotations
2. Register route in cmd/app/main.go
3. Add tests in internal/api/handlers_test.go
4. Run: "Generate Swagger Docs"
5. Run: "Run Tests"
6. Run: "Run App" and test
```

### Debugging Issues

```
1. Set breakpoint in code (click left gutter)
2. Run: "Docker Dependencies Only"
3. Debug (🐛): "Run App"
4. Trigger the issue (curl/Swagger UI)
5. Step through code
```

### Running Tests

```bash
# Unit tests
make test
# or Run Configuration: "Run Tests"

# With race detector
make test-race
# or Run Configuration: "Run Tests with Race Detector"

# With coverage
make test-cover

# Specific package
go test -v ./internal/api/...

# Specific test
go test -v -run TestHandleRequestUpdate ./internal/api/...
```

### Running Integration Tests

Integration tests live in `internal/integration/` and use the `//go:build integration` tag.
They require Postgres and Redis — either via testcontainers (automatic) or external services.

Requires Docker running. Testcontainers spins up ephemeral Postgres + Redis automatically:

```bash
make test-integration
# or Run Configuration: "Run Integration Tests"
```

To keep containers running after tests for inspection:

```bash
KEEP_CONTAINERS=1 make test-integration
```

### Code Quality Checks

```bash
# Format code
make fmt

# Run linter
make lint
# or Run Configuration: "Run Linter"

# All CI checks (fmt + vet + lint + test-race)
make ci
# or Run Configuration: "Run All CI Checks"
```

## Project Structure

```
exchange-rate-quote-service/
├── cmd/app/              # Application entry point
│   └── main.go          # Main + Swagger annotations
├── internal/
│   ├── api/             # HTTP handlers + middleware
│   │   ├── handlers.go  # All endpoint handlers
│   │   ├── middleware/
│   │   │   └── middleware.go # Request ID, logging
│   │   └── swagger.go   # Swagger UI setup
│   ├── config/          # Configuration loader
│   ├── provider/        # External API client
│   ├── repository/      # Database layer
│   │   └── migrations/  # SQL migrations
│   ├── service/         # Business logic
│   └── worker/          # Background job handler
├── .run/                # GoLand run configurations ⭐
├── .github/workflows/   # CI/CD pipeline
├── docker-compose.yml   # Local development setup
├── Dockerfile           # Application image
├── Makefile            # Development commands
└── README.md           # Main documentation
```

## Environment Variables

The "Run App" configuration includes these variables:

```bash
# Database
QUOTESVC_DATABASE_HOST=localhost
QUOTESVC_DATABASE_PORT=5432
QUOTESVC_DATABASE_USER=postgres
QUOTESVC_DATABASE_PASSWORD=postgres
QUOTESVC_DATABASE_NAME=quotesdb
QUOTESVC_DATABASE_SSLMODE=disable

# Redis
QUOTESVC_REDIS_ADDR=localhost:6380
QUOTESVC_REDIS_PASSWORD=

# Server
QUOTESVC_SERVER_PORT=8080
QUOTESVC_SERVER_SERVE_SWAGGER=true

# External API
QUOTESVC_EXTERNAL_BASE_URL=https://api.exchangerate.host
QUOTESVC_EXTERNAL_TIMEOUT_SEC=5

# Worker
QUOTESVC_WORKER_CONCURRENCY=1
```

To modify:
1. Run → Edit Configurations
2. Select "Run App"
3. Edit Environment variables

## Keyboard Shortcuts (GoLand)

| Action | macOS | Windows/Linux |
|--------|-------|---------------|
| Run | ⌃ R | Ctrl+Shift+F10 |
| Debug | ⌃ D | Shift+F9 |
| Run... (choose config) | ⌃ ⌥ R | Alt+Shift+F10 |
| Stop | ⌘ F2 | Ctrl+F2 |
| Toggle Breakpoint | ⌘ F8 | Ctrl+F8 |
| Step Over | F8 | F8 |
| Step Into | F7 | F7 |
| Resume | ⌘ ⌥ R | F9 |

## Troubleshooting

### "Cannot connect to database"

```bash
# Check if dependencies are running
docker compose ps

# If not running, start them
# Run Configuration: "Docker Dependencies Only"
# or
docker compose up db redis -d

# Check logs
docker compose logs db redis
```

### "Port 8080 already in use"

```bash
# Find process
lsof -i :8080

# Kill it
kill -9 <PID>

# Or change port in environment variables
```

### "Tests failing"

```bash
# Clean and re-download dependencies
go clean -cache
go mod tidy
go mod download

# Rebuild
make build
```

### "Swagger docs not updating"

```bash
# Regenerate
make swagger
# or Run Configuration: "Generate Swagger Docs"

# Check generated files
ls -la internal/api/docs/
```

### "Race detector fails"

Race conditions detected! This is good - it found a bug. Check the output:
- Which test failed
- Which goroutines are involved
- What data is being accessed concurrently

Fix the issue and run again.

## Git Workflow

```bash
# Before starting work
git checkout main
git pull

# Create feature branch
git checkout -b feature/your-feature

# Make changes...

# Before committing
make ci  # or Run Configuration: "Run All CI Checks"

# Commit
git add .
git commit -m "feat: your feature description"

# Push
git push origin feature/your-feature

# Create PR on GitHub
```

## Performance Tips

### Fast Test Iteration

```bash
# Run only changed package
go test ./internal/api/...

# Run specific test
go test -run TestHandleRequestUpdate ./internal/api/...

# Skip slow tests (if tagged)
go test -short ./...
```

### Fast Docker Rebuild

```bash
# Build only changed service
docker compose build app

# Start without build
docker compose up --no-build
```

### IDE Performance

- Enable "Build project automatically" for instant feedback
- Increase memory: Help → Change Memory Settings
- Exclude vendor/ and bin/ directories from indexing

## Resources

- [Main README](README.md) - Getting started
- [SPEC.md](SPEC.md) - Complete requirements
- [Makefile](Makefile) - All available make commands
- [.run/README.md](.run/README.md) - Run configurations details
- [Go Documentation](https://go.dev/doc/)
- [GoLand Documentation](https://www.jetbrains.com/help/go/)
