package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestRequestIDMiddleware(t *testing.T) {
	t.Run("generates UUID when no request ID provided", func(t *testing.T) {
		handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Context().Value(requestIDKey).(string)
			if reqID == "" {
				t.Error("Expected request ID in context, got empty string")
			}
			if _, err := uuid.Parse(reqID); err != nil {
				t.Errorf("Expected valid UUID, got: %s", reqID)
			}
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		respReqID := w.Header().Get(headerRequestID)
		if respReqID == "" {
			t.Error("Expected X-Request-Id in response header")
		}
		if _, err := uuid.Parse(respReqID); err != nil {
			t.Errorf("Expected valid UUID in response header, got: %s", respReqID)
		}
	})

	t.Run("uses provided request ID", func(t *testing.T) {
		providedID := "test-request-id-123"
		handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Context().Value(requestIDKey).(string)
			if reqID != providedID {
				t.Errorf("Expected request ID %s, got %s", providedID, reqID)
			}
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set(headerRequestID, providedID)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		respReqID := w.Header().Get(headerRequestID)
		if respReqID != providedID {
			t.Errorf("Expected X-Request-Id %s in response, got %s", providedID, respReqID)
		}
	})
}

func TestRequestLoggingMiddleware(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()

	handler := RequestLoggingMiddleware(sugar)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestResponseWriter(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, status: 0, size: 0}

	rw.WriteHeader(http.StatusCreated)
	if rw.status != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rw.status)
	}

	data := []byte("test data")
	n, err := rw.Write(data)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected %d bytes written, got %d", len(data), n)
	}
	if rw.size != len(data) {
		t.Errorf("Expected size %d, got %d", len(data), rw.size)
	}
}
