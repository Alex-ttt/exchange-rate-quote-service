// Package worker implements background task handlers for async quote processing.
package worker

import (
	"context"
	"encoding/json"
	"time"

	"quoteservice/internal/service"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// NewQuoteUpdateHandler returns a function to handle quote update tasks.
func NewQuoteUpdateHandler(svc service.QuoteServiceInterface, logger *zap.SugaredLogger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload service.UpdateQuotePayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			logger.Errorw("Invalid task payload", "type", t.Type(), "error", err)
			return nil
		}

		err := svc.ProcessUpdate(ctx, payload.UpdateID, payload.Base, payload.Quote)
		if err != nil {
			logger.Errorw("Task processing failed", "update_id", payload.UpdateID, "error", err)
			return err
		}

		logger.Infow("Task completed", "update_id", payload.UpdateID)
		return nil
	}
}

// AsynqEnqueuer is responsible for enqueuing tasks to an Asynq queue with specific configurations for retries and timeouts.
type AsynqEnqueuer struct {
	client   *asynq.Client
	maxRetry int
	timeout  time.Duration
}

// NewAsynqEnqueuer creates a new AsynqEnqueuer with the given client, retry limit, and task timeout duration.
func NewAsynqEnqueuer(client *asynq.Client, maxRetry int, timeout time.Duration) *AsynqEnqueuer {
	return &AsynqEnqueuer{
		client:   client,
		maxRetry: maxRetry,
		timeout:  timeout,
	}
}

// EnqueueUpdateTask enqueues a quote update task with the specified payload and context using Asynq.
func (e *AsynqEnqueuer) EnqueueUpdateTask(ctx context.Context, payload service.UpdateQuotePayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	task := asynq.NewTask(service.TaskTypeUpdateQuote, data,
		asynq.MaxRetry(e.maxRetry),
		asynq.Timeout(e.timeout),
	)

	_, err = e.client.EnqueueContext(ctx, task)
	return err
}
