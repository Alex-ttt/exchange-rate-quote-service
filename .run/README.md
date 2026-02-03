# GoLand Run Configurations

This directory contains pre-configured run configurations for GoLand/IntelliJ IDEA.

## Available Configurations

### Application

- **Run App** - Run the application locally with environment variables
  - Connects to localhost:5432 (Postgres), localhost:6380 (Redis Asynq) and localhost:6381 (Redis Cache)
  - Port: 8080
  - Swagger UI: http://localhost:8080/swagger/index.html

### Testing

- **Run Tests** - Execute all tests
- **Run Tests with Race Detector** - Run tests with race detection enabled

### Docker

- **Docker Compose Up** - Start all services (app + db + redis_asynq + redis_cache)
- **Docker Dependencies Only** - Start only db + redis_asynq + redis_cache (for local development)

### Build & Tools

- **Build Binary** - Build the application binary (`make build`)
- **Generate Swagger Docs** - Generate OpenAPI documentation (`make swagger`)
- **Run Linter** - Execute golangci-lint (`make lint`)
- **Run All CI Checks** - Run all CI checks locally (`make ci`)

## How to Use

### Method 1: From Run/Debug Dropdown

1. Click the Run/Debug dropdown in the top toolbar
2. Select the desired configuration
3. Click Run (▶️) or Debug (🐛)

### Method 2: Right-click Configuration

1. Right-click on any `.run.xml` file in the `.run/` directory
2. Select "Run '<configuration name>'"

## Local Development Workflow

### Option A: Full Docker (Recommended for beginners)

```
1. Run: "Docker Compose Up"
2. Access app at http://localhost:8080
```

### Option B: Local App + Docker Dependencies (Recommended for development)

```
1. Run: "Docker Dependencies Only"  (starts postgres + redis_asynq + redis_cache)
2. Run: "Run App"                   (starts the Go application)
3. Access app at http://localhost:8080
```

This allows you to:
- Use GoLand debugger
- Hot reload with code changes
- Better IDE integration

### Option C: Everything Local (Advanced)

Requires manually installed Postgres and Redis on your machine. Not recommended.

## Environment Variables

The **Run App** configuration includes all required environment variables:

```
QUOTESVC_DATABASE_HOST=localhost
QUOTESVC_DATABASE_PORT=5432
QUOTESVC_DATABASE_USER=postgres
QUOTESVC_DATABASE_PASSWORD=postgres
QUOTESVC_DATABASE_NAME=quotesdb
QUOTESVC_DATABASE_SSLMODE=disable
QUOTESVC_REDIS_ASYNQ_ADDR=localhost:6380
QUOTESVC_REDIS_CACHE_ADDR=localhost:6381
QUOTESVC_SERVER_PORT=8080
QUOTESVC_SERVER_SERVE_SWAGGER=true
QUOTESVC_EXTERNAL_BASE_URL=https://api.exchangerate.host
QUOTESVC_EXTERNAL_TIMEOUT_SEC=5
QUOTESVC_WORKER_CONCURRENCY=1
```

You can modify these in GoLand by:
1. Run → Edit Configurations
2. Select "Run App"
3. Modify Environment variables section

## Debugging

To debug the application:

1. Start: "Docker Dependencies Only"
2. Set breakpoints in the code
3. Click Debug (🐛) on "Run App"
4. Trigger requests to hit breakpoints

## Troubleshooting

### "Database connection failed"
- Ensure "Docker Dependencies Only" is running
- Check that ports 5432, 6380 and 6381 are not in use by other processes
- Verify Docker is running

### "Port already in use"
- Stop any other instance of the app
- Check if docker-compose is already running: `docker compose ps`
- Change port in environment variables if needed

### "Tests failing"
- Run `go mod download` to ensure dependencies are installed
- Run `go mod tidy` to clean up dependencies

## Tips

- Use "Run All CI Checks" before committing to ensure code quality
- Use "Run Tests with Race Detector" to catch concurrency issues
- Use "Generate Swagger Docs" after modifying API endpoints
- Keep "Docker Dependencies Only" running while developing locally
