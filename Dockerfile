# Build stage
FROM golang:1.25.6-alpine AS builder
WORKDIR /app

# Needed for go install / fetching modules
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for caching deps
COPY go.mod go.sum ./
RUN go mod download

# Install swag (for documentation generation)
RUN go install github.com/swaggo/swag/cmd/swag@master

# Copy the rest of the source
COPY . .

# Generate Swagger docs
RUN /go/bin/swag init -g cmd/app/main.go -o internal/api/docs

# Build the binary
RUN go build -o /app/quoteservice ./cmd/app


# Runtime stage
FROM alpine:3.23.3

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy binary and config
COPY --from=builder /app/quoteservice /app/quoteservice
COPY --from=builder /app/internal/config/config.yaml /app/config.yaml

# Set environment (if any defaults needed)
ENV CONFIG_FILE=/app/config.yaml

# Set permissions and user
RUN chown appuser:appgroup /app/quoteservice /app/config.yaml || true
USER appuser

EXPOSE 8080
ENTRYPOINT ["/app/quoteservice"]