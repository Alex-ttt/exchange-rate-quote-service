package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"quoteservice/internal/service"
)

// UpdateRequest represents the request body for quote update
type UpdateRequest struct {
	Pair string `json:"pair" example:"EUR/MXN"`
}

// UpdateResponse represents the response for a quote update request
type UpdateResponse struct {
	UpdateID string `json:"update_id" example:"123e4567-e89b-12d3-a456-426614174000"`
}

// QuoteResponse represents the response for a quote by ID
type QuoteResponse struct {
	UpdateID  string  `json:"update_id,omitempty" example:"123e4567-e89b-12d3-a456-426614174000"`
	Base      string  `json:"base" example:"EUR"`
	Quote     string  `json:"quote" example:"MXN"`
	Status    string  `json:"status" example:"SUCCESS"`
	Price     *string `json:"price,omitempty" example:"18.7543"`
	UpdatedAt *string `json:"updated_at,omitempty" example:"2025-12-01T10:15:30Z"`
	Error     *string `json:"error,omitempty" example:"Failed to fetch from provider"`
}

// LatestResponse represents the response for latest quote
type LatestResponse struct {
	Base      string `json:"base" example:"EUR"`
	Quote     string `json:"quote" example:"MXN"`
	Price     string `json:"price" example:"18.7543"`
	UpdatedAt string `json:"updated_at" example:"2025-12-01T10:15:30Z"`
}

// ReadyResponse represents the readiness response
type ReadyResponse struct {
	Status string `json:"status" example:"ready"`
}

// HandleRequestUpdate godoc
// @Summary Request asynchronous quote update
// @Description Initiates an asynchronous update for a currency pair. Returns immediately with an update_id for tracking. Does not block on external fetch.
// @Tags quotes
// @Accept json
// @Produce json
// @Param request body UpdateRequest true "Currency pair in format XXX/YYY"
// @Success 202 {object} UpdateResponse "Update request accepted"
// @Failure 400 {object} ErrorResponse "Invalid currency code format"
// @Failure 500 {object} ErrorResponse "Internal error"
// @Router /quotes/update [post]
func HandleRequestUpdate(svc service.QuoteServiceInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Pair string `json:"pair"`
		}
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		pair := strings.TrimSpace(req.Pair)
		if pair == "" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "pair is required"})
			return
		}
		updateID, _, err := svc.RequestQuoteUpdate(r.Context(), pair)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidPairFormat):
				writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			default:
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Internal error"})
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		resp := UpdateResponse{UpdateID: updateID}
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// HandleGetQuoteByID godoc
// @Summary Get quote update status and result by ID
// @Description Retrieves the status and result of a quote update request by its update_id. Returns price and timestamp when status is SUCCESS.
// @Tags quotes
// @Accept json
// @Produce json
// @Param update_id path string true "Update ID (UUID)" format(uuid)
// @Success 200 {object} QuoteResponse "Quote found"
// @Failure 400 {object} ErrorResponse "Invalid update_id format"
// @Failure 404 {object} ErrorResponse "Unknown update_id"
// @Failure 500 {object} ErrorResponse "Internal error"
// @Router /quotes/{update_id} [get]
func HandleGetQuoteByID(svc service.QuoteServiceInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		updateID := chi.URLParam(r, "update_id")
		if updateID == "" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "update_id is required"})
			return
		}

		quote, err := svc.GetQuoteResult(r.Context(), updateID)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidUpdateID):
				writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			case errors.Is(err, service.ErrNotFound):
				writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Unknown update_id"})
			default:
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Internal error"})
			}
			return
		}

		writeJSON(w, http.StatusOK, QuoteResponse{
			UpdateID:  quote.ID,
			Base:      quote.Base,
			Quote:     quote.Quote,
			Status:    quote.Status,
			Price:     quote.Price,
			UpdatedAt: quote.UpdatedAt,
			Error:     quote.ErrorMsg,
		})
	}
}

// HandleGetLatestQuote godoc
// @Summary Get latest quote for a currency pair
// @Description Returns the most recent successful quote for the given currency pair. Does NOT trigger a new fetch - only returns cached/stored data.
// @Tags quotes
// @Accept json
// @Produce json
// @Param base query string true "Base currency code (3 letters)" minlength(3) maxlength(3)
// @Param quote query string true "Quote currency code (3 letters)" minlength(3) maxlength(3)
// @Success 200 {object} LatestResponse "Latest quote found"
// @Failure 400 {object} ErrorResponse "Invalid currency code format"
// @Failure 404 {object} ErrorResponse "No quote available for the given pair"
// @Failure 500 {object} ErrorResponse "Internal error"
// @Router /quotes/latest [get]
func HandleGetLatestQuote(svc service.QuoteServiceInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := r.URL.Query().Get("base")
		quote := r.URL.Query().Get("quote")
		if base == "" || quote == "" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "base and quote query params are required"})
			return
		}
		latest, err := svc.GetLatestQuote(r.Context(), base, quote)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidPairFormat):
				writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			case errors.Is(err, service.ErrNotFound):
				writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "No quote available for " + strings.ToUpper(base) + "/" + strings.ToUpper(quote)})
			default:
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Internal error"})
			}
			return
		}

		writeJSON(w, http.StatusOK, LatestResponse{
			Base:      latest.Base,
			Quote:     latest.Quote,
			Price:     derefStr(latest.Price),
			UpdatedAt: derefStr(latest.UpdatedAt),
		})
	}
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
// @Description Checks connectivity to critical dependencies (Postgres and Redis). Returns 200 only when all dependencies are reachable.
// @Tags health
// @Produce json
// @Success 200 {object} ReadyResponse "All dependencies ready"
// @Failure 503 {object} ErrorResponse "At least one dependency unavailable"
// @Router /readyz [get]
func HandleReadyz(db *sql.DB, cache *redis.Client) http.HandlerFunc {
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

		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}
