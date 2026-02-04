package api

import (
	"database/sql"
	"net/http"

	"github.com/redis/go-redis/v9"
)

// ReadyResponse represents the readiness response
type ReadyResponse struct {
	Status string `json:"status" example:"ready"`
}

// HandleHealthz godoc
// @Summary Health check (liveness)
// @Description Always returns 200 OK if the service is running. Used for liveness probes.
// @Tags health
// @Produce plain
// @Success 200 {string} string "OK"
// @Router /healthz [get]
func HandleHealthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("OK"))
	}
}

// HandleReadyz godoc
// @Summary Readiness check
// @Description Checks connectivity to critical dependencies (Postgres, cache Redis, and asynq Redis). Returns 200 only when all dependencies are reachable.
// @Tags health
// @Produce json
// @Success 200 {object} ReadyResponse "All dependencies ready"
// @Failure 503 {object} ErrorResponse "At least one dependency unavailable"
// @Router /readyz [get]
func HandleReadyz(db *sql.DB, cache, asynqRedis *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "DB not ready"})
			return
		}

		if cache != nil {
			if err := cache.Ping(r.Context()).Err(); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "Cache not ready"})
				return
			}
		}

		if asynqRedis != nil {
			if err := asynqRedis.Ping(r.Context()).Err(); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "Asynq Redis not ready"})
				return
			}
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}
