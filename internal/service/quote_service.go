// Package service implements the core business logic for quote management.
package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"quoteservice/internal/config"
	"quoteservice/internal/provider"
	"quoteservice/internal/repository"
)

// QuoteServiceInterface defines the operations available for quote management.
type QuoteServiceInterface interface {
	RequestQuoteUpdate(ctx context.Context, pair string) (updateID, status string, err error)
	GetQuoteResult(ctx context.Context, updateID string) (*QuoteResult, error)
	GetLatestQuote(ctx context.Context, base, quote string) (*QuoteResult, error)
	ProcessUpdate(ctx context.Context, updateID, base, quote string) error
}

// QuoteService defines business logic for quotes
type QuoteService struct {
	repo         repository.QuoteRepository
	provider     provider.RatesProvider
	validator    Validator
	taskClient   *asynq.Client
	cache        *redis.Client
	log          *zap.SugaredLogger
	cacheTTL     time.Duration
	taskMaxRetry int
	taskTimeout  time.Duration
}

// NewQuoteService creates a new QuoteService
func NewQuoteService(repo repository.QuoteRepository, prov provider.RatesProvider, validator Validator, taskClient *asynq.Client, cache *redis.Client, logger *zap.SugaredLogger, cacheCfg config.CacheConfig) *QuoteService {
	return &QuoteService{
		repo:         repo,
		provider:     prov,
		validator:    validator,
		taskClient:   taskClient,
		cache:        cache,
		log:          logger,
		cacheTTL:     time.Duration(cacheCfg.TTLSec) * time.Second,
		taskMaxRetry: cacheCfg.TaskMaxRetry,
		taskTimeout:  time.Duration(cacheCfg.TaskTimeoutSec) * time.Second,
	}
}

// RequestQuoteUpdate processes a request to update a quote asynchronously.
func (s *QuoteService) RequestQuoteUpdate(ctx context.Context, pair string) (updateID, status string, err error) {
	base, quote, err := ParsePair(pair)
	if err != nil {
		return "", "", err
	}

	if vErr := s.validatePair(base, quote); vErr != nil {
		return "", "", vErr
	}

	uid := uuid.New().String()
	id, err := s.repo.CreateUpdate(ctx, base, quote, uid)
	if err != nil {
		s.log.Errorw("CreateUpdate DB error", "error", err)
		return "", "", ErrInternal
	}

	if id != uid {
		return id, string(repository.StatusPending), nil
	}

	if err := s.enqueueUpdateTask(ctx, id, base, quote); err != nil {
		return "", "", err
	}

	s.log.Infow("Enqueued update task", "update_id", id, "pair", base+"/"+quote)
	return id, string(repository.StatusPending), nil
}

// GetQuoteResult retrieves the quote (price and status) for a given update ID.
func (s *QuoteService) GetQuoteResult(ctx context.Context, updateID string) (*QuoteResult, error) {
	if _, err := uuid.Parse(updateID); err != nil {
		return nil, ErrInvalidUpdateID
	}
	q, err := s.repo.GetByID(ctx, updateID)
	if err != nil {
		s.log.Errorw("DB error fetching quote by ID", "update_id", updateID, "error", err)
		return nil, ErrInternal
	}
	if q == nil {
		return nil, ErrNotFound
	}

	return quoteResultFromRepo(q), nil
}

// GetLatestQuote returns the latest successful quote for the given currency pair.
func (s *QuoteService) GetLatestQuote(ctx context.Context, base, quote string) (*QuoteResult, error) {
	base, quote, err := normalizePair(base, quote)
	if err != nil {
		return nil, err
	}

	if vErr := s.validatePair(base, quote); vErr != nil {
		return nil, vErr
	}

	if q, ok := s.cacheGetLatest(ctx, base, quote); ok {
		return quoteResultFromRepo(q), nil
	}

	q, err := s.repo.GetLatestSuccess(ctx, base, quote)
	if err != nil {
		s.log.Errorw("DB error fetching latest quote", "base", base, "quote", quote, "error", err)
		return nil, ErrInternal
	}
	if q == nil {
		return nil, ErrNotFound
	}

	s.cacheSetLatestFromQuote(ctx, q)
	return quoteResultFromRepo(q), nil
}

// ProcessUpdate performs the external fetch and updates the result (called by background worker).
func (s *QuoteService) ProcessUpdate(ctx context.Context, updateID, base, quote string) error {
	base, quote, err := normalizePair(base, quote)
	if err != nil {
		return err
	}

	if vErr := s.validatePair(base, quote); vErr != nil {
		s.completeFailure(ctx, updateID, vErr)
		return vErr
	}

	s.log.Infow("Processing update", "update_id", updateID, "base", base, "quote", quote)
	s.markRunning(ctx, updateID)

	rate, fetchedAt, err := s.provider.GetRate(base, quote)
	if err != nil {
		s.completeFailure(ctx, updateID, err)
		return err
	}

	if err := s.repo.MarkCompleted(ctx, updateID, rate, repository.StatusSuccess, nil); err != nil {
		s.log.Errorw("DB update error on success", "update_id", updateID, "error", err)
		return err
	}

	s.cacheSetLatest(ctx, base, quote, rate, fetchedAt)
	s.log.Infow("Update success", "update_id", updateID, "rate", rate)
	return nil
}

func (s *QuoteService) enqueueUpdateTask(ctx context.Context, updateID, base, quote string) error {
	task, err := s.createUpdateQuoteTask(updateID, base, quote)
	if err != nil {
		s.log.Errorw("Failed to create task payload", "error", err)
		s.markFailed(ctx, updateID, "task creation error")
		return ErrInternal
	}

	if _, err := s.taskClient.Enqueue(task); err != nil {
		s.log.Errorw("Failed to enqueue task", "update_id", updateID, "error", err)
		s.markFailed(ctx, updateID, "enqueue error")
		return ErrInternalQueue
	}
	return nil
}

func (s *QuoteService) markFailed(ctx context.Context, updateID, reason string) {
	if err := s.repo.MarkCompleted(ctx, updateID, "", repository.StatusFailed, strPtr(reason)); err != nil {
		s.log.Warnw("Failed to mark record as FAILED", "update_id", updateID, "error", err)
	}
}

func (s *QuoteService) markRunning(ctx context.Context, updateID string) {
	if err := s.repo.MarkRunning(ctx, updateID); err != nil {
		s.log.Warnw("Failed to mark record as RUNNING", "update_id", updateID, "error", err)
	}
}

func (s *QuoteService) completeFailure(ctx context.Context, updateID string, cause error) {
	s.log.Errorw("Provider error", "update_id", updateID, "error", cause)
	msg := cause.Error()
	if err := s.repo.MarkCompleted(ctx, updateID, "", repository.StatusFailed, &msg); err != nil {
		s.log.Warnw("Failed to mark record as FAILED after provider error", "update_id", updateID, "error", err)
	}
}

// TaskTypeUpdateQuote is the Asynq task type for quote update jobs.
const TaskTypeUpdateQuote = "quote:update"

// UpdateQuotePayload is the payload structure for quote update Asynq tasks.
type UpdateQuotePayload struct {
	UpdateID string `json:"update_id"`
	Base     string `json:"base"`
	Quote    string `json:"quote"`
}

// createUpdateQuoteTask creates an Asynq Task for updating a quote.
func (s *QuoteService) createUpdateQuoteTask(updateID, base, quote string) (*asynq.Task, error) {
	payload, err := json.Marshal(UpdateQuotePayload{
		UpdateID: updateID,
		Base:     base,
		Quote:    quote,
	})
	if err != nil {
		return nil, err
	}
	task := asynq.NewTask(TaskTypeUpdateQuote, payload,
		asynq.MaxRetry(s.taskMaxRetry),
		asynq.Timeout(s.taskTimeout),
	)
	return task, nil
}

func (s *QuoteService) validatePair(base, quote string) error {
	if err := s.validator.Validate(base); err != nil {
		return err
	}
	err := s.validator.Validate(quote)
	return err
}
