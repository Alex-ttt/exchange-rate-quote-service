package service

import (
	"time"

	"quoteservice/internal/repository"
)

// QuoteResult represents a quote result returned by the service layer.
// Fields are populated according to the quote's status:
//   - SUCCESS: Price and UpdatedAt are set, ErrorMsg is nil.
//   - FAILED:  ErrorMsg is set, Price is nil.
//   - PENDING/RUNNING: Price, ErrorMsg, and UpdatedAt are nil.
type QuoteResult struct {
	ID        string
	Base      string
	Quote     string
	Price     *string
	Status    string
	ErrorMsg  *string
	UpdatedAt *string
}

func quoteResultFromRepo(q *repository.Quote) *QuoteResult {
	r := &QuoteResult{
		ID:     q.ID,
		Base:   q.Base,
		Quote:  q.Quote,
		Status: string(q.Status),
	}

	switch q.Status {
	case repository.StatusSuccess:
		r.Price = q.Price
		if q.UpdatedAt != nil {
			ts := q.UpdatedAt.Format(time.RFC3339)
			r.UpdatedAt = &ts
		}
	case repository.StatusFailed:
		r.ErrorMsg = q.ErrorMsg
	}

	return r
}
