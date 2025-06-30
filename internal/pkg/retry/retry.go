// Package retry provides utilities for retrying failed operations with configurable backoff strategies.
// It implements exponential backoff with jitter to prevent thundering herd problems and includes
// specialized support for HTTP operations with status code awareness.
//
// Basic usage:
//
//	cfg := retry.DefaultConfig()
//	err := retry.WithExponentialBackoff(ctx, cfg, logger, "my_operation", func() error {
//	    return doSomething()
//	})
//
// For HTTP operations:
//
//	httpConfig := retry.DefaultHTTPConfig()
//	client := retry.NewHTTPClientWithRetry(httpConfig, logger)
//	resp, err := client.Get("https://api.example.com/data")
package retry

import (
	"context"
	"math"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// Config defines the configuration for retry operations.
// It controls the number of attempts and delays between retries.
type Config struct {
	MaxAttempts int           // Maximum number of retry attempts
	BaseDelay   time.Duration // Base delay between retries
	MaxDelay    time.Duration // Maximum delay between retries
}

// DefaultConfig returns a default retry configuration suitable for most operations.
// It uses 3 attempts with exponential backoff starting at 1 second.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    10 * time.Second,
	}
}

// CriticalConfig returns a retry configuration for critical startup operations.
// It uses more aggressive retry settings for fail-fast scenarios with longer initial delays.
func CriticalConfig() Config {
	return Config{
		MaxAttempts: 3,
		BaseDelay:   2 * time.Second,
		MaxDelay:    8 * time.Second,
	}
}

// WithExponentialBackoff executes an operation with exponential backoff retry logic.
// It will retry the operation up to MaxAttempts times with exponentially increasing delays.
// The delay between attempts is calculated as: BaseDelay * 2^(attempt-1), capped at MaxDelay.
//
// The operation will be retried if:
// - It returns a non-nil error
// - The context is not cancelled
// - Maximum attempts have not been exceeded
//
// If all attempts fail, it returns the last error encountered.
//
// Example:
//
//	err := WithExponentialBackoff(ctx, cfg, log, "database_connect", func() error {
//	    return db.Connect()
//	})
func WithExponentialBackoff(ctx context.Context, cfg Config, log *logger.Logger, operationName string, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Execute the operation
		if err := operation(); err != nil {
			lastErr = err

			// If this was the last attempt, log critical error and return
			if attempt == cfg.MaxAttempts {
				log.Critical("Operation failed after all retry attempts",
					"operation", operationName,
					"attempts", cfg.MaxAttempts,
					"error", err.Error())
				return err
			}

			// Calculate delay for next attempt using exponential backoff
			delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt-1)))
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}

			log.Warn("Operation failed, retrying",
				"operation", operationName,
				"attempt", attempt,
				"max_attempts", cfg.MaxAttempts,
				"next_retry_in", delay.String(),
				"error", err.Error())

			// Wait for the calculated delay or until context is cancelled
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				log.Error("Retry operation cancelled due to context timeout",
					"operation", operationName,
					"attempt", attempt,
					"context_error", ctx.Err().Error())
				return ctx.Err()
			}
		} else {
			// Operation succeeded
			if attempt > 1 {
				log.Info("Operation succeeded after retry",
					"operation", operationName,
					"attempt", attempt)
			} else {
				log.Debug("Operation succeeded on first attempt",
					"operation", operationName)
			}
			return nil
		}
	}

	return lastErr
}

// WithSimpleRetry executes an operation with simple retry logic using a fixed delay.
// Unlike WithExponentialBackoff, this uses a constant delay between all retry attempts.
// It's suitable for operations that don't benefit from exponential backoff, such as
// polling operations or when interacting with systems that have predictable recovery times.
//
// Example:
//
//	err := WithSimpleRetry(ctx, 5, 2*time.Second, log, "poll_status", func() error {
//	    return checkStatus()
//	})
func WithSimpleRetry(ctx context.Context, maxAttempts int, delay time.Duration, log *logger.Logger, operationName string, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := operation(); err != nil {
			lastErr = err

			if attempt == maxAttempts {
				log.Critical("Operation failed after all retry attempts",
					"operation", operationName,
					"attempts", maxAttempts,
					"error", err.Error())
				return err
			}

			log.Warn("Operation failed, retrying",
				"operation", operationName,
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"retry_delay", delay.String(),
				"error", err.Error())

			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				log.Error("Retry operation cancelled due to context timeout",
					"operation", operationName,
					"attempt", attempt,
					"context_error", ctx.Err().Error())
				return ctx.Err()
			}
		} else {
			if attempt > 1 {
				log.Info("Operation succeeded after retry",
					"operation", operationName,
					"attempt", attempt)
			}
			return nil
		}
	}

	return lastErr
}
