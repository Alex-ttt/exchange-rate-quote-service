package service

import (
	"errors"
	"strings"
)

var supportedCurrencies = map[string]struct{}{
	"USD": {},
	"EUR": {},
	"GBP": {},
	"JPY": {},
	"CHF": {},
	"CAD": {},
	"AUD": {},
	"NZD": {},
	"CNY": {},
	"HKD": {},
	"SGD": {},
	"SEK": {},
	"NOK": {},
	"INR": {},
	"MXN": {},
}

// ErrUnsupportedCurrency is returned when a currency is not in the supported list.
var ErrUnsupportedCurrency = errors.New("unsupported currency")

// Validator defines the interface for currency validation.
type Validator interface {
	Validate(code string) error
	IsSupported(code string) bool
}

type validator struct{}

// NewValidator creates a new currency validator.
func NewValidator() Validator {
	return &validator{}
}

// Validate checks if the currency code is supported (case-insensitive).
func (v *validator) Validate(code string) error {
	if v.IsSupported(code) {
		return nil
	}
	return ErrUnsupportedCurrency
}

// IsSupported returns true if the currency code is supported (case-insensitive).
func (v *validator) IsSupported(code string) bool {
	_, ok := supportedCurrencies[strings.ToUpper(code)]
	return ok
}
