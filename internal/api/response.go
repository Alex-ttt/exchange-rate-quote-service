// Package api implements HTTP handlers for the exchange rate quote service.
package api

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error" example:"Invalid currency code format"`
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// derefStr returns the string value of a pointer, or an empty string if nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
