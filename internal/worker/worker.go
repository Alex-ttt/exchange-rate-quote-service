// Package worker implements background task handlers for async quote processing.
package worker

import (
	"context"
	"encoding/json"

	"quoteservice/internal/service"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// NewQuoteUpdateHandler returns a function to handle quote update tasks.
func NewQuoteUpdateHandler(svc *service.QuoteService, logger *zap.SugaredLogger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var p service.UpdateQuotePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			logger.Errorw("Invalid task payload", "type", t.Type(), "error", err)
			return nil
		}

		err := svc.ProcessUpdate(ctx, p.UpdateID, p.Base, p.Quote)
		if err != nil {
			logger.Errorw("Task processing failed", "update_id", p.UpdateID, "error", err)
			return err
		}

		logger.Infow("Task completed", "update_id", p.UpdateID)
		return nil
	}
}
