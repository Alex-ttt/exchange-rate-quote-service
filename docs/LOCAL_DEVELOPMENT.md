# Local Development Setup

Complete guide for running the Currency Quotes Service locally in GoLand.

## Prerequisites

- ✅ GoLand 2023.3+ or IntelliJ IDEA with Go plugin
- ✅ Docker Desktop installed and running
- ✅ Go 1.25.6+ installed

## Quick Start (2 Steps)

### Step 1: Start Dependencies

Run the **"Docker Dependencies Only"** configuration:
- Click Run dropdown → "Docker Dependencies Only"
- Or press **⌃⌥R** (macOS) / **Alt+Shift+F10** (Windows) and select it

This starts:
- PostgreSQL on `localhost:5432`
- Redis on `localhost:6380`

**Verify they're running:**
```bash
docker compose ps
# Should show db and redis with status "Up"
```

### Step 2: Run the Application

Run the **"Run App"** configuration:
- Click Run dropdown → "Run App"
- Click the green ▶️ button

The application starts on `http://localhost:8080`

**Test it works:**
```bash
curl http://localhost:8080/healthz
# Should return: OK
```

---

## Understanding the Fix

### Problem
When running locally, the app tried to connect to hostname `db` (Docker container name) instead of `localhost`.

### Root Cause
Viper's `AutomaticEnv()` doesn't automatically map nested config keys like `database.host` to environment variables without explicit configuration.

### Solution
Added `SetEnvKeyReplacer` to config loader:

**File:** `internal/config/config.go`
```go
viper.SetEnvPrefix("QUOTESVC")
viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
viper.AutomaticEnv()
```

This maps:
- Config key: `database.host`
- To env var: `QUOTESVC_DATABASE_HOST`

### Environment Variable Mapping

The "Run App" configuration sets these env vars:

| Config Key | Environment Variable | Value | Why? |
|-----------|---------------------|--------|------|
| `database.host` | `QUOTESVC_DATABASE_HOST` | `localhost` | Connect to local Docker |
| `database.port` | `QUOTESVC_DATABASE_PORT` | `5432` | Standard Postgres port |
| `redis.addr` | `QUOTESVC_REDIS_ADDR` | `localhost:6380` | Connect to local Docker |
| `server.port` | `QUOTESVC_SERVER_PORT` | `8080` | App listens here |
| `external.timeout_sec` | `QUOTESVC_EXTERNAL_TIMEOUT_SEC` | `5` | External API timeout in seconds |

---

## Development Workflow

### Daily Workflow

```
1. Open GoLand
   └─ Project loads automatically

2. Start Dependencies (once per day)
   └─ Run: "Docker Dependencies Only"
   └─ Postgres + Redis start
   └─ Leave running

3. Run Application (as needed)
   └─ Run/Debug: "Run App"
   └─ App connects to localhost:5432, localhost:6380
   └─ Make changes → Restart app

4. Test Changes
   └─ Swagger UI: http://localhost:8080/swagger/index.html
   └─ Health: http://localhost:8080/healthz
   └─ Or use curl

5. Run Tests (after changes)
   └─ Run: "Run Tests"
   └─ Or: "Run Tests with Race Detector"

6. Before Committing
   └─ Run: "Run All CI Checks"
   └─ Fix any issues
   └─ Commit
```

### Debugging Workflow

```
1. Set Breakpoints
   └─ Click line numbers to add breakpoints

2. Start Dependencies
   └─ Run: "Docker Dependencies Only"

3. Debug Application
   └─ Click Debug button (🐛) on "Run App"
   └─ Or press ⌃D (macOS) / Shift+F9 (Windows)

4. Trigger Code
   └─ Make API request (curl, Swagger UI, etc.)
   └─ Debugger stops at breakpoint

5. Step Through Code
   └─ F8: Step Over
   └─ F7: Step Into
   └─ F9: Resume
```

---

## Configuration Files

### .run/Run App.run.xml

The "Run App" configuration is stored here and includes:

```xml
<envs>
  <env name="QUOTESVC_DATABASE_HOST" value="localhost" />
  <env name="QUOTESVC_DATABASE_PORT" value="5432" />
  <env name="QUOTESVC_DATABASE_USER" value="postgres" />
  <env name="QUOTESVC_DATABASE_PASSWORD" value="postgres" />
  <env name="QUOTESVC_DATABASE_NAME" value="quotesdb" />
  <env name="QUOTESVC_DATABASE_SSLMODE" value="disable" />
  <env name="QUOTESVC_REDIS_ADDR" value="localhost:6380" />
  <env name="QUOTESVC_SERVER_PORT" value="8080" />
  <!-- ... more env vars -->
</envs>
```

**To modify:**
1. Run → Edit Configurations
2. Select "Run App"
3. Edit Environment variables section
4. Click OK

### internal/config/config.yaml

Default configuration used when environment variables are NOT set:

```yaml
database:
  host: db              # Docker container name
  port: 5432

redis:
  addr: redis:6380      # Docker container name
```

**Environment variables override these defaults!**

---

## Verification Steps

### 1. Check Config Loading

Add temporary logging to see what config is loaded:

**File:** `internal/config/config.go`
```go
func LoadConfig() (*Config, error) {
    // ... existing code ...

    // Temporary debug log
    fmt.Printf("Database Host: %s\n", cfg.Database.Host)
    fmt.Printf("Redis Addr: %s\n", cfg.Redis.Addr)

    return &cfg, nil
}
```

Run "Run App" and check output:
```
Database Host: localhost  ← Should be localhost, not "db"
Redis Addr: localhost:6380
```

### 2. Check Docker Dependencies

```bash
# Should show 2 containers running
docker compose ps

# Check Postgres
docker compose exec db psql -U postgres -c "SELECT version();"

# Check Redis
docker compose exec redis redis-cli -p 6380 ping
# Should return: PONG
```

### 3. Test Connectivity

```bash
# Test Postgres
nc -zv localhost 5432
# Output: Connection to localhost port 5432 [tcp/*] succeeded!

# Test Redis
nc -zv localhost 6380
# Output: Connection to localhost port 6380 [tcp/*] succeeded!
```

---

## Troubleshooting

### Still Getting "lookup db: no such host"?

1. **Verify environment variables are set in Run Configuration:**
   - Run → Edit Configurations → "Run App"
   - Check Environment variables section
   - Ensure `QUOTESVC_DATABASE_HOST=localhost` is there

2. **Check config loader has key replacer:**
   ```bash
   grep "SetEnvKeyReplacer" internal/config/config.go
   ```
   Should return a line with `strings.NewReplacer(".", "_")`

3. **Test with explicit env var:**
   ```bash
   QUOTESVC_DATABASE_HOST=localhost go run ./cmd/app
   ```

4. **Check Docker dependencies are running:**
   ```bash
   docker compose ps | grep -E "(db|redis)"
   ```

### Migration Error: "syntax error at or near NOT"

**Fixed!** The migration now uses a DO block for enum creation:

```sql
DO $$ BEGIN
    CREATE TYPE quotes_status AS ENUM ('PENDING', 'RUNNING', 'SUCCESS', 'FAILED');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;
```

If you still see this error:
1. Drop the database: `docker compose down -v`
2. Restart: `docker compose up db redis -d`
3. Run app again

---

## Alternative: Command Line Development

If you prefer terminal over IDE:

```bash
# Terminal 1: Start dependencies
docker compose up db redis

# Terminal 2: Run app with env vars
export QUOTESVC_DATABASE_HOST=localhost
export QUOTESVC_REDIS_ADDR=localhost:6380
go run ./cmd/app

# Or one-liner:
QUOTESVC_DATABASE_HOST=localhost \
QUOTESVC_REDIS_ADDR=localhost:6380 \
go run ./cmd/app
```

---

## Next Steps

1. ✅ Dependencies running
2. ✅ App running locally
3. ✅ Environment variables working
4. → Start developing!

**Useful Links:**
- Swagger UI: http://localhost:8080/swagger/index.html
- Health Check: http://localhost:8080/healthz
- Readiness: http://localhost:8080/readyz

**Quick Tests:**
```bash
# Health check
curl http://localhost:8080/healthz

# Request quote update
curl -X POST http://localhost:8080/quotes/update \
  -H 'Content-Type: application/json' \
  -d '{"pair":"EUR/MXN"}'

# Get latest quote (after successful update)
curl "http://localhost:8080/quotes/latest?base=EUR&quote=MXN"
```

Happy coding! 🚀
