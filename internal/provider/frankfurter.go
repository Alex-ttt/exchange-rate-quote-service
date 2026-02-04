package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

var _ RatesProvider = (*FrankfurterProvider)(nil)

// FrankfurterProvider fetches rates from the Frankfurter API.
type FrankfurterProvider struct {
	baseURL string
	client  *http.Client
}

// NewFrankfurterProvider creates a new FrankfurterProvider.
func NewFrankfurterProvider(baseURL string, timeoutSec int) *FrankfurterProvider {
	if baseURL == "" {
		baseURL = "https://api.frankfurter.dev/v1"
	}
	return &FrankfurterProvider{
		baseURL: baseURL,
		client:  &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
	}
}

type frankfurterResponse struct {
	Amount float64            `json:"amount"`
	Base   string             `json:"base"`
	Date   string             `json:"date"`
	Rates  map[string]float64 `json:"rates"`
}

// GetRate retrieves the exchange rate between the specified base and quote currencies
func (p *FrankfurterProvider) GetRate(ctx context.Context, base, quote string) (string, time.Time, error) {
	reqURL := fmt.Sprintf("%s/latest?base=%s&symbols=%s", p.baseURL, base, quote)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("frankfurter API request creation failed: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("frankfurter API request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("frankfurter API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result frankfurterResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to decode frankfurter API response: %w", err)
	}

	rateVal, ok := result.Rates[quote]
	if !ok {
		return "", time.Time{}, fmt.Errorf("no rate for %s in frankfurter response", quote)
	}

	rateStr := strconv.FormatFloat(rateVal, 'f', -1, 64)

	// Parse date from response if possible, otherwise use current time
	resDate, err := time.Parse("2006-01-02", result.Date)
	if err != nil {
		return rateStr, time.Now().UTC(), nil
	}

	return rateStr, resDate.UTC(), nil
}
