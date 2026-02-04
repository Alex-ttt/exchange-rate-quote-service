// Package provider implements external rate providers for fetching currency exchange rates.
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

var _ RatesProvider = (*ExchangeRateHostProvider)(nil)

// ExchangeRateHostProvider fetches rates from the exchangerate.host API.
type ExchangeRateHostProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewExchangeRateHostProvider creates a new ExchangeRateHostProvider with the given configuration.
func NewExchangeRateHostProvider(baseURL, apiKey string, timeoutSec int) *ExchangeRateHostProvider {
	if baseURL == "" {
		baseURL = "https://api.exchangerate.host"
	}
	return &ExchangeRateHostProvider{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
	}
}

// getLatestURL forms the API URL for fetching the rate.
func (p *ExchangeRateHostProvider) getLatestURL(base, quote string) string {
	return fmt.Sprintf("%s/live?access_key=%s&source=%s&currencies=%s",
		p.baseURL, p.apiKey, base, quote)
}

// exchangerate.host latest API response structure
type erHostResponse struct {
	Success bool               `json:"success"`
	Source  string             `json:"source"`
	Quotes  map[string]float64 `json:"quotes"`
}

// GetRate fetches the exchange rate for the given base/quote currency pair.
func (p *ExchangeRateHostProvider) GetRate(ctx context.Context, base, quote string) (string, time.Time, error) {
	reqURL := p.getLatestURL(base, quote)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("external API request creation failed: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("external API request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("external API returned status %d: %s", resp.StatusCode, string(body))
	}
	var result erHostResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to decode external API response: %w", err)
	}
	if !result.Success {
		return "", time.Time{}, fmt.Errorf("external API returned success=false for %s/%s", base, quote)
	}
	// The API returns quotes keyed as "BASEQUOTE", e.g. "EURMXN"
	key := base + quote
	rateVal, ok := result.Quotes[key]
	if !ok {
		return "", time.Time{}, fmt.Errorf("no rate for %s in response", key)
	}
	rateStr := strconv.FormatFloat(rateVal, 'f', -1, 64)
	return rateStr, time.Now().UTC(), nil
}
