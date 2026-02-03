## Goal

Build a Go HTTP/JSON service that manages **asynchronous** currency quote updates:

- Client requests an update for a currency pair (e.g. `EUR/MXN`) and receives an `update_id` immediately.
- The actual fetch from an external rates provider happens **in the background**.
- Client can poll by `update_id` to get status/result, or request the latest known quote for a pair.
- Service exposes health endpoints and production-grade observability (request id + structured logs).
- Runs locally with Docker Compose (Postgres + Redis + app), one command.

---

## Functional Requirements

### FR-1: Request quote update (async)
- Accept a request to update a quote for a currency pair.
- Return quickly without performing external fetch inline.
- Return `update_id` (UUID) for tracking.

### FR-2: Get quote update result by `update_id`
- Return status and, when ready, the fetched price and update time.
- If still processing or failed, return status accordingly.

### FR-3: Get latest quote for a currency pair
- Return the most recent successful quote stored for the pair (does **not** trigger external fetch).

### FR-4: Health endpoints
- `GET /healthz` (liveness): indicates the process is running.
- `GET /readyz` (readiness): checks connectivity to critical deps (Postgres and Redis).

### FR-5: Limited currency support (demo)
- The service is intended to demonstrate a limited set of currencies/pairs.
- **Assumption (from provided design):** accept any ISO-4217-like 3-letter codes; external provider ultimately determines support. Tests should focus on a few (e.g. USD/EUR/MXN).

---

## Non-Functional Requirements

### Reliability
- Background updates should be executed **at-least-once**.
- External API failures must be handled gracefully (mark update as `FAILED`).

### Performance
- `POST /quotes/update` should be fast (enqueue only).
- `GET /quotes/latest` should be optimized (use Redis cache).

### Timeouts & retries
- External provider calls: timeout ~ **3–5s**.
- Retries: exponential backoff for transient errors (network/5xx/429), with a max attempts limit.
- DB/Redis calls: should use context timeouts (no indefinite hangs).

### Rate limiting / dedup
- Avoid frequent redundant fetches for the same pair.
- Deduplicate in-flight updates per pair.

### Observability
- Correlation/request id middleware:
    - Accept incoming request id header if present, else generate one.
    - Propagate it via context and response header.
- Structured JSON logs:
    - Required fields: `timestamp`, `level`, `msg`, `request_id`, `method`, `path`, `status`, `duration_ms`.
    - Background worker logs should include at least `update_id` (and request_id if propagated).

### Security & secrets
- Secrets must not be hard-coded; use env vars.
- Container should run as **non-root** user.

### Scalability & concurrency
- Design should support multiple instances:
    - worker coordination via Redis-backed queue (at-least-once),
    - consistency via DB constraints/transactions.

---

## Asynchronous Update State Machine

### States
- `PENDING`: created, queued.
- `RUNNING`: picked up by worker (optional but recommended).
- `SUCCESS`: price + updated_at available.
- `FAILED`: failed to fetch/store.

### Transitions
- `PENDING -> RUNNING -> SUCCESS/FAILED`
- Retries may occur internally; final state becomes `FAILED` after max retries.

### Semantics when result is not ready
- For `PENDING`/`RUNNING`: return response with `status` and no `price/updated_at` (or null).

### Deduplication (Idempotency)
- If an update for the same pair is already `PENDING` or `RUNNING`,
    - return the existing `update_id` instead of starting a new fetch.

---

## API

- Base content type: `application/json` (except `/healthz` may be text/plain per design).
- Currency pair format: `"XXX/YYY"` where `XXX` and `YYY` are `[A-Z]{3}`.

### 1) POST `/quotes/update`
Request an asynchronous update.

**Request body**
```json
{ "pair": "EUR/MXN" }
```

**Validation**
- `pair` is required.
- Must match `^[A-Z]{3}/[A-Z]{3}$`.

**Responses**
- `202 Accepted`:
```json
{ "update_id": "123e4567-e89b-12d3-a456-426614174000", "status": "PENDING" }
```
- `status` is optional but recommended.

- `400 Bad Request` (invalid pair):
```json
{ "error": "Invalid currency code format" }
```

- `500 Internal Server Error` (unexpected/infra issue):
```json
{ "error": "Internal error" }
```

**Idempotency behavior**
- If in-flight update exists for pair, return same `update_id` with `202`.

---

### 2) GET `/quotes/{update_id}`
Get update status/result by id.

**Path params**
- `update_id`: UUID

**Validation**
- Must be a valid UUID format; else `400`.

**Responses**
- `200 OK` (found):
    - `SUCCESS`:
```json
{
  "update_id": "123e4567-e89b-12d3-a456-426614174000",
  "base": "EUR",
  "quote": "MXN",
  "status": "SUCCESS",
  "price": "18.7543",
  "updated_at": "2025-12-01T10:15:30Z"
}
```
- `PENDING` / `RUNNING`:
```json
{ "update_id": "123e4567-e89b-12d3-a456-426614174000", "status": "PENDING" }
```
- `FAILED`:
```json
{
  "update_id": "123e4567-e89b-12d3-a456-426614174000",
  "status": "FAILED",
  "error": "Failed to fetch from provider"
}
```

- `400 Bad Request` (invalid UUID):
```json
{ "error": "Invalid update_id" }
```

- `404 Not Found` (unknown id):
```json
{ "error": "Unknown update_id" }
```

---

### 3) GET `/quotes/latest`
Get latest stored quote for a pair (no external fetch).

**Query params**
- `base` (required): 3-letter code
- `quote` (required): 3-letter code

**Assumption (from provided design):**
- Use `?base=EUR&quote=MXN` (preferred) rather than `pair=EUR/MXN`.

**Validation**
- `base`, `quote` must match `^[A-Z]{3}$`.

**Responses**
- `200 OK`:
```json
{
  "base": "EUR",
  "quote": "MXN",
  "price": "18.7543",
  "updated_at": "2025-12-01T10:15:30Z"
}
```

- `400 Bad Request`:
```json
{ "error": "Invalid currency code format" }
```

- `404 Not Found` (no successful quote stored):
```json
{ "error": "No quote available for EUR/MXN" }
```

---

### 4) GET `/healthz` (liveness)
- Always returns `200 OK` if process is running.

**Response (per design)**
- `text/plain`: `OK`
    - **Clarification needed:** if implementation uses JSON instead, it should still be `200` and stable.

---

### 5) GET `/readyz` (readiness)
- Checks connectivity to Postgres and Redis.

**Responses**
- `200 OK`:
```json
{ "status": "ready" }
```
- `503 Service Unavailable` (preferred) if dependency down:
```json
{ "error": "Not ready: postgres unavailable" }
```

---

## Data Storage

### Postgres schema (expected)
A single table holding update requests and results.

**Table: `quotes`**
- `id UUID PRIMARY KEY`
- `base CHAR(3) NOT NULL`
- `quote CHAR(3) NOT NULL`
- `price NUMERIC(18,6) NULL`
- `status VARCHAR(10) NOT NULL` in `{PENDING, RUNNING, SUCCESS, FAILED}`
- `error TEXT NULL`
- `requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NULL`

**Indexes**
- Latest retrieval:
    - `INDEX (base, quote, updated_at DESC)`
- Deduplication for in-flight updates:
    - Partial unique index:
        - `UNIQUE (base, quote) WHERE status IN ('PENDING','RUNNING')`

**Price representation**
- Stored as `NUMERIC(18,6)`.
- API returns `price` as a **string** (decimal string).

---

## Background Processing

### Task queue
- Redis-backed job queue using **Asynq**.
- Enqueue on `POST /quotes/update`.
- Worker fetches external rate and updates DB status transitions:
    - `PENDING -> RUNNING -> SUCCESS/FAILED`
- External call has timeout (~3–5s) and retries (Asynq retry policy and/or explicit backoff for transient errors).

---

## Caching (Redis)

### What is cached
- Latest quote per pair:
    - Key format example: `latest:EUR:MXN` (implementation choice; must be consistent).
    - Value includes `price` and `updated_at`.

### Read policy
- `GET /quotes/latest`: read-through cache
    - cache hit -> return
    - cache miss -> read from DB -> set cache -> return

### Write policy
- On successful update -> set cache for that pair.

### TTL / eviction
- TTL may be set (implementation-defined).
- Redis eviction must not break correctness; DB remains source of truth.

---

## Observability

### Correlation / request id
- Middleware behavior:
    - Accept header (recommended name): `X-Request-Id`
    - If absent, generate UUID.
    - Add to response header `X-Request-Id`.
    - Store in request context for downstream usage.

### Structured logs
- Use Zap (per design).
- One JSON log line per event.
- Required fields (HTTP logs):
    - `timestamp`, `level`, `msg`,
    - `request_id`, `method`, `path`, `status`, `duration_ms`
- Worker logs should include:
    - at least `update_id`, `base`, `quote`, and `level/timestamp/msg`.

---

## Configuration

### Docker images (fixed versions)
- Postgres: `postgres:18.1-alpine`
- Redis: `redis:8.4.0-alpine`

### Redis port
- Redis must run on a **custom port** (example: `6380`) in Docker Compose.

### Required environment variables (minimum set)
**Clarification needed:** exact names may vary; repository must document and align env vars across compose/config code.

Recommended minimal set:
- `HTTP_ADDR` (default `:8080`)
- `POSTGRES_DSN` (or `DB_DSN`)
- `REDIS_ADDR` (e.g. `redis:6380`)
- `REDIS_PASSWORD` (optional; likely empty for local)
- `RATES_PROVIDER_BASE_URL` (e.g. exchangerate.host base URL)

Secrets:
- DB password must be via env (compose).

Public config:
- Optional YAML config file (e.g. `config.yaml`) may exist; env overrides it.

---

## Local Run

### One-command startup
- `docker compose up --build`

Expected services:
- `app` (HTTP API + worker)
- `postgres` (18.1-alpine)
- `redis` (8.4.0-alpine, port 6380)

### Typical scenarios

#### Scenario A: Update + poll by id
1) Request update:
```bash
curl -s -X POST http://localhost:8080/quotes/update \
  -H 'Content-Type: application/json' \
  -d '{"pair":"EUR/MXN"}'
```
2) Poll:
```bash
curl -s http://localhost:8080/quotes/<update_id>
```

#### Scenario B: Get latest
```bash
curl -s "http://localhost:8080/quotes/latest?base=EUR&quote=MXN"
```

#### Scenario C: Health checks
```bash
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/readyz
```

---

## Acceptance Criteria (Definition of Done)

1. **Build & run**
    - `docker compose up --build` starts Postgres, Redis (port 6380), and app successfully.
    - App serves HTTP on configured port.

2. **API contracts**
    - `POST /quotes/update` returns `202` with `update_id`.
    - `GET /quotes/{update_id}` returns `200` with correct `status` and fields per status.
    - `GET /quotes/latest` returns latest quote or `404` if none.
    - Validation errors return `400` with `{ "error": "..." }`.
    - Unknown `update_id` returns `404`.

3. **Async behavior**
    - Update request does not block on external fetch.
    - Background worker updates DB and cache; status progresses to `SUCCESS` or `FAILED`.

4. **Deduplication**
    - Concurrent/rapid repeated `POST /quotes/update` for the same pair returns the same in-flight `update_id` (while status is `PENDING`/`RUNNING`).

5. **Health endpoints**
    - `/healthz` returns `200` consistently.
    - `/readyz` returns `200` only when Postgres and Redis are reachable; otherwise `503` (or `500` if implemented—should be consistent and documented).

6. **Observability**
    - If request lacks `X-Request-Id`, service generates one and returns it in response header.
    - Structured request logs include required fields, especially `request_id`.

7. **Tests**
    - `go test ./...` passes.
    - Unit tests cover validation, dedup/idempotency behavior, worker processing paths, cache hit/miss, request id middleware.

---

## Known ambiguities / open questions

1. **/healthz response format**
    - Design shows `text/plain "OK"`. If implementation uses JSON, ensure consistency and document it.

2. **Retention policy**
    - Design assumes update records are retained indefinitely. No cleanup/TTL is specified for DB rows.

3. **Exact env var names**
    - The design states config via env + optional YAML, but exact variable names are not defined. Repo should align compose, config loader, and documentation.

4. **TTL for cached latest quotes**
    - TTL is discussed as optional; exact TTL value is not specified and may be left as a default/constant.
