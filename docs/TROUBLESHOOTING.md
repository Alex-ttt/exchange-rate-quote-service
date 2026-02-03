# Troubleshooting Guide

Common issues and solutions when running the Currency Quotes Service.

## Local Development Issues

### Issue: "hostname resolving error: lookup db: no such host"

**Error Message:**
```
Failed to connect to Postgres: unable to connect to database:
hostname resolving error: lookup db: no such host
```

**Cause:**
The application is trying to connect to hostname "db" (Docker container name) instead of "localhost" when running locally.

**Solution:**

The environment variables in the "Run App" configuration should properly override the config file values. If this error occurs:

1. **Verify Docker dependencies are running:**
   ```bash
   docker compose ps
   # You should see db and redis running
   ```

2. **If not running, start them:**
   - In GoLand: Run configuration "Docker Dependencies Only"
   - Or via command line: `docker compose up db redis -d`

3. **Verify environment variables in Run Configuration:**
   - Go to: Run → Edit Configurations → "Run App"
   - Check Environment variables section contains:
     ```
     QUOTESVC_DATABASE_HOST=localhost
     QUOTESVC_DATABASE_PORT=5432
     QUOTESVC_REDIS_ADDR=localhost:6380
     ```

4. **If environment variables are correct but still failing:**

   The config loader uses Viper with automatic environment variable mapping:
   - Config key: `database.host`
   - Maps to env var: `QUOTESVC_DATABASE_HOST`

   Ensure the `SetEnvKeyReplacer` is present in `internal/config/config.go`:
   ```go
   viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
   ```

5. **Test environment variable override:**
   ```bash
   # Set env var and run
   export QUOTESVC_DATABASE_HOST=localhost
   go run ./cmd/app
   ```

**Quick Fix:**
Stop the app and run:
```bash
# Terminal 1: Start dependencies
docker compose up db redis

# Terminal 2: Run app with explicit env vars
QUOTESVC_DATABASE_HOST=localhost \
QUOTESVC_REDIS_ADDR=localhost:6380 \
go run ./cmd/app
```

---

### Issue: "Port already in use"

**Error Message:**
```
bind: address already in use
```

**Solutions:**

1. **Find what's using the port:**
   ```bash
   # macOS/Linux
   lsof -i :8080

   # Windows
   netstat -ano | findstr :8080
   ```

2. **Kill the process:**
   ```bash
   # macOS/Linux
   kill -9 <PID>

   # Windows
   taskkill /PID <PID> /F
   ```

3. **Or change the port:**
   - Edit Run Configuration environment variables
   - Add: `QUOTESVC_SERVER_PORT=8081`

---

### Issue: "Database connection refused"

**Error Message:**
```
connection refused
```

**Solutions:**

1. **Check if Postgres is running:**
   ```bash
   docker compose ps db
   ```

2. **Check Postgres logs:**
   ```bash
   docker compose logs db
   ```

3. **Verify port is accessible:**
   ```bash
   nc -zv localhost 5432
   # or
   telnet localhost 5432
   ```

4. **Restart Postgres:**
   ```bash
   docker compose restart db
   ```

---

### Issue: "Redis connection refused"

**Error Message:**
```
dial tcp [::1]:6380: connect: connection refused
```

**Solutions:**

1. **Check if Redis is running:**
   ```bash
   docker compose ps redis
   ```

2. **Verify Redis is on port 6380:**
   ```bash
   docker compose logs redis | grep "Ready to accept connections"
   ```

3. **Test Redis connection:**
   ```bash
   docker compose exec redis redis-cli -p 6380 ping
   # Should return: PONG
   ```

4. **Restart Redis:**
   ```bash
   docker compose restart redis
   ```

---

### Issue: Tests Failing

**Error Message:**
```
--- FAIL: TestXxx (0.00s)
```

**Solutions:**

1. **Clean test cache:**
   ```bash
   go clean -testcache
   go test ./...
   ```

2. **Update dependencies:**
   ```bash
   go mod tidy
   go mod download
   ```

3. **Check for race conditions:**
   ```bash
   go test -race ./...
   ```

---

### Issue: Swagger Docs Not Updating

**Problem:**
API changes don't appear in Swagger UI.

**Solutions:**

1. **Regenerate Swagger docs:**
   - Run Configuration: "Generate Swagger Docs"
   - Or: `make swagger`
   - Or: `swag init -g cmd/app/main.go -o internal/api/docs`

2. **Clear browser cache:**
   - Hard refresh: Cmd+Shift+R (Mac) or Ctrl+Shift+R (Windows)
   - Or open in incognito mode

3. **Restart the application:**
   - Stop and start the "Run App" configuration

---

### Issue: "panic: runtime error: invalid memory address"

**Error Message:**
```
panic: runtime error: invalid memory address or nil pointer dereference
```

**Common Causes:**

1. **Nil pointer in service/repository:**
   - Check if all dependencies are properly initialized in main.go
   - Verify mock interfaces in tests

2. **Missing environment variables:**
   - Check all required env vars are set
   - Review `.env.example` for complete list

**Debug Steps:**
1. Enable debugging in GoLand
2. Set breakpoint at panic location
3. Inspect variable values

---

### Issue: Migration Errors

**Error Message:**
```
Failed to run DB migrations: execute migration 001_init.sql: ...
```

**Solutions:**

1. **Drop and recreate database:**
   ```bash
   docker compose down -v
   docker compose up db -d
   # Wait for DB to be ready
   docker compose up app
   ```

2. **Check migration file:**
   ```bash
   cat internal/repository/migrations/001_init.sql
   ```

3. **Manually run migration:**
   ```bash
   docker compose exec db psql -U postgres -d quotesdb -f /path/to/migration.sql
   ```

---

## Docker Issues

### Issue: "Cannot connect to Docker daemon"

**Solutions:**

1. **Start Docker Desktop:**
   - macOS: Open Docker Desktop app
   - Linux: `sudo systemctl start docker`
   - Windows: Start Docker Desktop

2. **Verify Docker is running:**
   ```bash
   docker ps
   ```

---

### Issue: "No space left on device"

**Solutions:**

1. **Clean up Docker:**
   ```bash
   docker system prune -a --volumes
   ```

2. **Remove unused images:**
   ```bash
   docker image prune -a
   ```

---

### Issue: Slow Docker Builds

**Solutions:**

1. **Use BuildKit:**
   ```bash
   DOCKER_BUILDKIT=1 docker compose build
   ```

2. **Clean build cache:**
   ```bash
   docker builder prune
   ```

---

## Build Issues

### Issue: "undefined: strings.NewReplacer"

**Error Message:**
```
internal/config/config.go:XX: undefined: strings
```

**Solution:**
Add missing import:
```go
import (
    "strings"
    // ... other imports
)
```

---

### Issue: Module Version Conflicts

**Error Message:**
```
go: inconsistent vendoring
```

**Solutions:**

1. **Clean and update:**
   ```bash
   go clean -modcache
   rm go.sum
   go mod tidy
   ```

2. **Verify module file:**
   ```bash
   go mod verify
   ```

---

## IDE Issues (GoLand)

### Issue: Run Configurations Not Appearing

**Solutions:**

1. **Invalidate caches:**
   - File → Invalidate Caches → Invalidate and Restart

2. **Reimport project:**
   - Close project
   - Delete `.idea/` folder (if exists locally)
   - Reopen project

3. **Verify .run/ directory exists:**
   ```bash
   ls -la .run/
   ```

---

### Issue: Debugger Not Stopping at Breakpoints

**Solutions:**

1. **Rebuild project:**
   - Build → Rebuild Project

2. **Check debug mode:**
   - Use Debug (🐛) button, not Run (▶️)

3. **Verify breakpoint is set:**
   - Red dot should appear in gutter
   - Right-click breakpoint for conditions

---

## Performance Issues

### Issue: Slow Application Startup

**Possible Causes:**

1. **Database connection timeout:**
   - Check if Postgres is ready
   - Increase timeout in config

2. **Network issues:**
   - Check Docker network
   - Verify DNS resolution

**Solutions:**
```bash
# Check Docker network
docker network inspect exchange-rate-quote-service_default

# Check DNS
docker compose exec app nslookup db
```

---

### Issue: High Memory Usage

**Solutions:**

1. **Check running processes:**
   ```bash
   docker stats
   ```

2. **Limit Docker resources:**
   - Docker Desktop → Settings → Resources
   - Adjust memory limits

---

## Environment Variable Issues

### Issue: Environment Variables Not Being Read

**Debug Steps:**

1. **Print environment variables:**
   Add to config.go:
   ```go
   fmt.Printf("DATABASE_HOST: %s\n", os.Getenv("QUOTESVC_DATABASE_HOST"))
   ```

2. **Check variable naming:**
   - Must start with `QUOTESVC_`
   - Use underscores for nested: `QUOTESVC_DATABASE_HOST`

3. **Verify in Run Configuration:**
   - Run → Edit Configurations
   - Check Environment variables section

---

---

## Getting Help

### Logs to Collect

When reporting issues, include:

1. **Application logs:**
   ```bash
   docker compose logs app
   ```

2. **Dependency logs:**
   ```bash
   docker compose logs db redis
   ```

3. **Build output:**
   ```bash
   go build -v ./cmd/app 2>&1 | tee build.log
   ```

4. **Environment info:**
   ```bash
   go version
   docker version
   docker compose version
   ```

### Useful Commands

```bash
# Full system check
make ci

# Clean everything and restart
docker compose down -v
rm -rf bin/ internal/api/docs/
go clean -cache -testcache -modcache
go mod download
docker compose up --build

# Run with verbose logging
go run ./cmd/app -v

# Test single package
go test -v ./internal/api/...

# Profile the application
go test -cpuprofile=cpu.prof -memprofile=mem.prof ./...
go tool pprof cpu.prof
```

---

## Still Having Issues?

1. Check [README.md](../README.md) for basic setup
2. Review [DEVELOPMENT.md](../DEVELOPMENT.md) for workflows
3. Check [GitHub Issues](https://github.com/yourusername/exchange-rate-quote-service/issues)
4. Ask in team chat or create an issue
