// Package middleware provides HTTP middleware for request ID tracking and logging.
package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type contextKey string

const requestIDKey contextKey = "request_id"
const headerRequestID = "X-Request-Id"

// RequestIDMiddleware ensures each request has a correlation ID
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(headerRequestID)
		if reqID == "" {
			reqID = uuid.New().String()
		}
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		w.Header().Set(headerRequestID, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestLoggingMiddleware logs each HTTP request and response details
func RequestLoggingMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, status: 0, size: 0}
			next.ServeHTTP(ww, r)
			duration := time.Since(start)
			reqID, _ := r.Context().Value(requestIDKey).(string)
			if ww.status == 0 {
				ww.status = 200
			}
			logger.Infow("HTTP request",
				"request_id", reqID,
				"method", r.Method,
				"path", r.RequestURI,
				"status", ww.status,
				"duration_ms", duration.Milliseconds(),
			)
		})
	}
}

// responseWriter is a wrapper to capture HTTP status and size
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

// WriteHeader captures status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}
