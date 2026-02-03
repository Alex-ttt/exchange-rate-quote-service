package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"quoteservice/internal/service"
)

func TestHandleRequestUpdate(t *testing.T) {
	t.Run("valid pair returns 202", func(t *testing.T) {
		svc := &mockQuoteService{
			requestUpdateFunc: func(ctx context.Context, pair string) (string, string, error) {
				return "test-uuid-123", "PENDING", nil
			},
		}

		body := bytes.NewBufferString(`{"pair":"EUR/MXN"}`)
		req := httptest.NewRequest(http.MethodPost, "/quotes/update", body)
		w := httptest.NewRecorder()

		handler := HandleRequestUpdate(svc)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("Expected status 202, got %d", w.Code)
		}

		var resp UpdateResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.UpdateID != "test-uuid-123" {
			t.Errorf("Expected update_id 'test-uuid-123', got %s", resp.UpdateID)
		}
	})

	t.Run("invalid pair format returns 400", func(t *testing.T) {
		svc := &mockQuoteService{
			requestUpdateFunc: func(ctx context.Context, pair string) (string, string, error) {
				return "", "", service.ErrInvalidPairFormat
			},
		}

		body := bytes.NewBufferString(`{"pair":"INVALID"}`)
		req := httptest.NewRequest(http.MethodPost, "/quotes/update", body)
		w := httptest.NewRecorder()

		handler := HandleRequestUpdate(svc)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		expectedError := "invalid currency code format"
		if resp.Error != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, resp.Error)
		}
	})

	t.Run("missing pair returns 400", func(t *testing.T) {
		svc := &mockQuoteService{}

		body := bytes.NewBufferString(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/quotes/update", body)
		w := httptest.NewRecorder()

		handler := HandleRequestUpdate(svc)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

func TestHandleGetQuoteByID(t *testing.T) {
	t.Run("success status returns full quote", func(t *testing.T) {
		price := "18.7543"
		updatedAt := "2025-12-01T10:15:30Z"
		svc := &mockQuoteService{
			getQuoteResultFunc: func(ctx context.Context, updateID string) (*service.QuoteResult, error) {
				return &service.QuoteResult{
					ID:        "test-uuid",
					Base:      "EUR",
					Quote:     "MXN",
					Status:    "SUCCESS",
					Price:     &price,
					UpdatedAt: &updatedAt,
				}, nil
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/quotes/test-uuid", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("update_id", "test-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		handler := HandleGetQuoteByID(svc)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp QuoteResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Status != "SUCCESS" {
			t.Errorf("Expected status SUCCESS, got %s", resp.Status)
		}
		if resp.Price == nil || *resp.Price != price {
			t.Errorf("Expected price %s, got %v", price, resp.Price)
		}
		if resp.UpdatedAt == nil {
			t.Error("Expected updated_at to be present")
		}
	})

	t.Run("pending status returns no price", func(t *testing.T) {
		svc := &mockQuoteService{
			getQuoteResultFunc: func(ctx context.Context, updateID string) (*service.QuoteResult, error) {
				return &service.QuoteResult{
					ID:     "test-uuid",
					Base:   "EUR",
					Quote:  "MXN",
					Status: "PENDING",
				}, nil
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/quotes/test-uuid", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("update_id", "test-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		handler := HandleGetQuoteByID(svc)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp QuoteResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Status != "PENDING" {
			t.Errorf("Expected status PENDING, got %s", resp.Status)
		}
		if resp.Price != nil {
			t.Error("Expected price to be nil for PENDING status")
		}
	})

	t.Run("invalid UUID returns 400", func(t *testing.T) {
		svc := &mockQuoteService{
			getQuoteResultFunc: func(ctx context.Context, updateID string) (*service.QuoteResult, error) {
				return nil, service.ErrInvalidUpdateID
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/quotes/invalid", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("update_id", "invalid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		handler := HandleGetQuoteByID(svc)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("unknown ID returns 404", func(t *testing.T) {
		svc := &mockQuoteService{
			getQuoteResultFunc: func(ctx context.Context, updateID string) (*service.QuoteResult, error) {
				return nil, service.ErrNotFound
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/quotes/unknown-uuid", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("update_id", "unknown-uuid")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		handler := HandleGetQuoteByID(svc)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Error != "Unknown update_id" {
			t.Errorf("Expected error 'Unknown update_id', got '%s'", resp.Error)
		}
	})
}

func TestHandleGetLatestQuote(t *testing.T) {
	t.Run("valid pair returns latest quote", func(t *testing.T) {
		price := "18.7543"
		updatedAt := "2025-12-01T10:15:30Z"
		svc := &mockQuoteService{
			getLatestQuoteFunc: func(ctx context.Context, base, quote string) (*service.QuoteResult, error) {
				return &service.QuoteResult{
					Base:      base,
					Quote:     quote,
					Price:     &price,
					UpdatedAt: &updatedAt,
					Status:    "SUCCESS",
				}, nil
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/quotes/latest?base=EUR&quote=MXN", nil)
		w := httptest.NewRecorder()

		handler := HandleGetLatestQuote(svc)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp LatestResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Base != "EUR" || resp.Quote != "MXN" {
			t.Errorf("Expected EUR/MXN, got %s/%s", resp.Base, resp.Quote)
		}
		if resp.Price != price {
			t.Errorf("Expected price %s, got %s", price, resp.Price)
		}
	})

	t.Run("missing query params returns 400", func(t *testing.T) {
		svc := &mockQuoteService{}

		req := httptest.NewRequest(http.MethodGet, "/quotes/latest", nil)
		w := httptest.NewRecorder()

		handler := HandleGetLatestQuote(svc)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("no quote available returns 404", func(t *testing.T) {
		svc := &mockQuoteService{
			getLatestQuoteFunc: func(ctx context.Context, base, quote string) (*service.QuoteResult, error) {
				return nil, service.ErrNotFound
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/quotes/latest?base=EUR&quote=MXN", nil)
		w := httptest.NewRecorder()

		handler := HandleGetLatestQuote(svc)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Error != "No quote available for EUR/MXN" {
			t.Errorf("Expected specific error message, got '%s'", resp.Error)
		}
	})
}

func TestHandleHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handler := HandleHealthz()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", w.Body.String())
	}
}
