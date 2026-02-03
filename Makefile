.PHONY: help build test test-integration test-integration-ci test-race test-cover lint fmt vet docker-build docker-up docker-down clean swagger run

# Variables
BINARY_NAME=quoteservice
DOCKER_IMAGE=quoteservice:latest
COVERAGE_FILE=coverage.out

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application binary
	@echo "Building $(BINARY_NAME)..."
	go build -v -o ./bin/$(BINARY_NAME) ./cmd/app

test: ## Run tests
	@echo "Running tests..."
	go test -v ./...

test-integration: ## Run integration tests (uses testcontainers, requires Docker)
	@echo "Running integration tests..."
	go test -tags integration -v -count=1 ./internal/integration/...

test-integration-ci: ## Run integration tests against external Postgres/Redis (set TEST_PG_DSN, TEST_REDIS_ADDR)
	@echo "Running integration tests against external services..."
	go test -tags integration -v -count=1 -race ./internal/integration/...


test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	go test -race -v ./...

test-cover: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint: ## Run linter
	@echo "Running golangci-lint..."
	golangci-lint run --timeout=5m

fmt: ## Format code
	@echo "Formatting code..."
	gofmt -s -w .
	goimports -w -local quoteservice .

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

swagger: ## Generate swagger documentation
	@echo "Generating swagger docs..."
	swag init -g cmd/app/main.go -o internal/api/docs
	@echo "Swagger docs generated in internal/api/docs/"

docker-build: ## Build docker image
	@echo "Building docker image..."
	docker compose build

docker-up: ## Start docker compose services
	@echo "Starting services..."
	docker compose up -d
	@echo "Services started. API available at http://localhost:8080"
	@echo "Swagger UI: http://localhost:8080/swagger/index.html"

docker-down: ## Stop docker compose services
	@echo "Stopping services..."
	docker compose down -v

docker-logs: ## Show docker compose logs
	docker compose logs -f

run: ## Run the application locally (requires DB and Redis running)
	@echo "Starting $(BINARY_NAME)..."
	go run ./cmd/app

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf ./bin
	rm -f $(COVERAGE_FILE) coverage.html
	rm -rf ./internal/api/docs
	go clean

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	go mod verify

install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

ci: fmt vet lint test-race ## Run all CI checks locally
	@echo "All CI checks passed!"

.DEFAULT_GOAL := help
