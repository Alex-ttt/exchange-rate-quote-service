package service

import (
	"errors"
	"strings"
)

func normalizePair(base, quote string) (normBase, normQuote string, err error) {
	if !IsValidCurrencyCode(base) || !IsValidCurrencyCode(quote) {
		return "", "", ErrInvalidPairFormat
	}
	return strings.ToUpper(base), strings.ToUpper(quote), nil
}

// ErrInvalidPairFormat indicates the currency pair format is invalid.
var ErrInvalidPairFormat = errors.New("invalid currency code format")

// ErrInvalidUpdateID indicates the update ID format is invalid.
var ErrInvalidUpdateID = errors.New("invalid update_id")

// ErrNotFound indicates the requested resource was not found.
var ErrNotFound = errors.New("not found")

// ErrInternal indicates an internal server error.
var ErrInternal = errors.New("internal error")

// ErrInternalQueue indicates an internal queue error.
var ErrInternalQueue = errors.New("internal queue error")

// IsValidCurrencyCode checks whether a string is a valid 3-letter currency code.
func IsValidCurrencyCode(code string) bool {
	if len(code) != 3 {
		return false
	}
	code = strings.ToUpper(code)
	for _, c := range code {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// ParsePair splits a "BASE/QUOTE" string into its components and validates them.
func ParsePair(pair string) (base, quote string, err error) {
	parts := strings.Split(pair, "/")
	if len(parts) != 2 || !IsValidCurrencyCode(parts[0]) || !IsValidCurrencyCode(parts[1]) {
		return "", "", ErrInvalidPairFormat
	}
	return strings.ToUpper(parts[0]), strings.ToUpper(parts[1]), nil
}
