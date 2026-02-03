# Swagger/OpenAPI Documentation Setup

Complete guide for Swagger UI in the Currency Quotes Service.

## Quick Access

Once the application is running:
- **Swagger UI**: http://localhost:8080/swagger/index.html
- **OpenAPI JSON**: http://localhost:8080/openapi.json (redirects to `/swagger/doc.json`)
- **Raw Spec**: http://localhost:8080/swagger/doc.json

## Setup Complete ✅

The Swagger documentation is now fully configured:

1. ✅ Swagger annotations added to all handlers
2. ✅ Documentation generated in `internal/api/docs/`
3. ✅ Docs package imported in `main.go`
4. ✅ Swagger UI routes registered
5. ✅ Build successful

## How It Works

### 1. Annotations in Handlers

All endpoints have Swagger annotations:

**Example from `internal/api/handlers.go`:**
```go
// HandleRequestUpdate godoc
// @Summary Request asynchronous quote update
// @Description Initiates an asynchronous update for a currency pair
// @Tags quotes
// @Accept json
// @Produce json
// @Param request body UpdateRequest true "Currency pair in format XXX/YYY"
// @Success 202 {object} UpdateResponse "Update request accepted"
// @Failure 400 {object} ErrorResponse "Invalid currency code format"
// @Failure 500 {object} ErrorResponse "Internal error"
// @Router /quotes/update [post]
func HandleRequestUpdate(svc service.QuoteServiceInterface) http.HandlerFunc {
    // ... implementation
}
```

### 2. Generated Documentation

The `swag init` command generates:
- `internal/api/docs/docs.go` - Go code with swagger spec
- `internal/api/docs/swagger.json` - OpenAPI JSON spec
- `internal/api/docs/swagger.yaml` - OpenAPI YAML spec

### 3. Main.go Configuration

```go
// @title Currency Quotes Service API
// @version 1.0
// @description Asynchronous currency quote update service
// @host localhost:8080
// @BasePath /
package main

import (
    _ "quoteservice/internal/api/docs" // Import generated docs
    // ... other imports
)
```

### 4. Routes

```go
// Serve Swagger UI and spec
if cfg.Server.ServeSwagger {
    r.Get("/swagger/*", api.SwaggerUIHandler())
    r.Get("/openapi.json", api.OpenAPISpecHandler())
}
```

## Regenerating Documentation

### When to Regenerate

Regenerate Swagger docs whenever you:
- Add new endpoints
- Modify endpoint parameters
- Change response schemas
- Update descriptions or examples

### How to Regenerate

**Option 1: GoLand Run Configuration**
- Run: "Generate Swagger Docs"

**Option 2: Makefile**
```bash
make swagger
```

**Option 3: Direct Command**
```bash
swag init -g cmd/app/main.go -o internal/api/docs --parseDependency
```

### After Regenerating

1. **Rebuild the app:**
   ```bash
   go build ./cmd/app
   ```

2. **Restart the application:**
   - Stop and start the "Run App" configuration
   - Or `make run`

3. **Hard refresh browser:**
   - Cmd+Shift+R (Mac)
   - Ctrl+Shift+R (Windows)
   - Or open in incognito mode

## Swagger UI Features

### Interactive Testing

1. **Expand an endpoint** (e.g., POST /quotes/update)

2. **Click "Try it out"**

3. **Edit the request body:**
   ```json
   {
     "pair": "EUR/MXN"
   }
   ```

4. **Click "Execute"**

5. **View the response:**
   - Status code
   - Response body
   - Headers

### Example Requests

The Swagger UI includes example values for all schemas:

```json
{
  "pair": "EUR/MXN"
}
```

### Authentication

Currently the API doesn't require authentication. If added in the future, update:

```go
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
```

Then on protected endpoints:
```go
// @Security ApiKeyAuth
```

## Swagger Annotations Reference

### General Info (in main.go)

```go
// @title Currency Quotes Service API
// @version 1.0
// @description Asynchronous currency quote update service
// @host localhost:8080
// @BasePath /
```

### Endpoint Annotations

```go
// @Summary Short description (< 120 chars)
// @Description Detailed description
// @Tags group-name
// @Accept json
// @Produce json
// @Param name path string true "Description"
// @Param name query string false "Description"
// @Param request body ModelName true "Description"
// @Success 200 {object} ResponseModel
// @Failure 400 {object} ErrorResponse
// @Router /path [get]
```

### Parameter Types

- `path` - URL path parameter (e.g., `/quotes/{id}`)
- `query` - Query string parameter (e.g., `?base=EUR`)
- `body` - Request body
- `header` - HTTP header

### Response Types

- `{object}` - JSON object
- `{array}` - JSON array
- `{string}` - Plain text

### Common Patterns

**Path parameter:**
```go
// @Param id path string true "Resource ID" format(uuid)
```

**Query parameter:**
```go
// @Param base query string true "Base currency" minlength(3) maxlength(3)
```

**Request body:**
```go
// @Param request body UpdateRequest true "Update request"
```

**Multiple success responses:**
```go
// @Success 200 {object} SuccessResponse
// @Success 201 {object} CreatedResponse
```

**Multiple failure responses:**
```go
// @Failure 400 {object} ErrorResponse "Invalid input"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal error"
```

## Model Examples

Models are defined in handler responses with example tags:

```go
type UpdateRequest struct {
    Pair string `json:"pair" example:"EUR/MXN"`
}

type UpdateResponse struct {
    UpdateID string `json:"update_id" example:"123e4567-e89b-12d3-a456-426614174000"`
    Status   string `json:"status" example:"PENDING"`
}
```

## Troubleshooting

### Issue: "Cannot GET /swagger/index.html"

**Cause:** Swagger docs not generated or import missing.

**Solution:**
```bash
# Regenerate docs
swag init -g cmd/app/main.go -o internal/api/docs

# Verify import in main.go
grep "internal/api/docs" cmd/app/main.go
# Should show: _ "quoteservice/internal/api/docs"

# Rebuild
go build ./cmd/app
```

### Issue: "404 Not Found" on Swagger UI

**Cause:** `ServeSwagger` config is false.

**Solution:**
- Check `config.yaml`: `serve_swagger: true`
- Or set env var: `QUOTESVC_SERVER_SERVE_SWAGGER=true`

### Issue: Changes Not Showing

**Cause:** Browser cache or docs not regenerated.

**Solutions:**
1. Regenerate docs: `make swagger`
2. Rebuild app: `go build ./cmd/app`
3. Restart app
4. Hard refresh browser: Cmd+Shift+R

### Issue: Build Error After Generating Docs

**Cause:** Version mismatch between swag CLI and library.

**Solution:**
```bash
# Update dependencies
go get -u github.com/swaggo/swag@latest
go mod tidy

# Regenerate
rm -rf internal/api/docs
swag init -g cmd/app/main.go -o internal/api/docs

# Rebuild
go build ./cmd/app
```

### Issue: "unknown field LeftDelim"

**Fixed!** This was caused by version incompatibility. Now resolved by:
1. Updating swag dependencies
2. Regenerating docs with `--parseDependency` flag

## CI/CD Integration

### Docker Build

The Dockerfile already includes Swagger generation:

```dockerfile
# Install swag (for documentation generation)
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Generate Swagger docs
RUN /go/bin/swag init -g cmd/app/main.go -o internal/api/docs
```

### GitHub Actions

The CI pipeline can include a docs check:

```yaml
- name: Verify Swagger docs
  run: |
    swag init -g cmd/app/main.go -o internal/api/docs
    git diff --exit-code internal/api/docs/
```

## Best Practices

### 1. Keep Annotations Updated

When modifying an endpoint, update its annotations immediately.

### 2. Use Descriptive Examples

```go
// Good
Price string `json:"price" example:"18.7543"`

// Bad
Price string `json:"price" example:"123"`
```

### 3. Document All Error Cases

```go
// @Failure 400 {object} ErrorResponse "Invalid currency code format"
// @Failure 404 {object} ErrorResponse "Unknown update_id"
// @Failure 500 {object} ErrorResponse "Internal error"
```

### 4. Group Related Endpoints

```go
// @Tags quotes
// @Tags health
```

### 5. Add Descriptions to Request/Response Models

```go
// UpdateRequest represents the request body for quote update
type UpdateRequest struct {
    // Currency pair in format XXX/YYY
    Pair string `json:"pair" example:"EUR/MXN"`
}
```

## Next Steps

1. **Access Swagger UI:**
   - Start app: "Run App" configuration
   - Open: http://localhost:8080/swagger/index.html

2. **Try the API:**
   - Expand POST /quotes/update
   - Click "Try it out"
   - Execute request
   - See live response

3. **Modify and Regenerate:**
   - Change an annotation
   - Run: "Generate Swagger Docs"
   - Restart app
   - See updated docs

Happy documenting! 📚
