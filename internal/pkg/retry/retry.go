package retry

import (
	"context"
	"math"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// Config defines the configuration for retry operations
type Config struct {
	MaxAttempts int           // Maximum number of retry attempts
	BaseDelay   time.Duration // Base delay between retries
	MaxDelay    time.Duration // Maximum delay between retries
}

// DefaultConfig returns a default retry configuration suitable for most operations
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    10 * time.Second,
	}
}

// CriticalConfig returns a retry configuration for critical startup operations
// Uses more aggressive retry settings for fail-fast scenarios
func CriticalConfig() Config {
	return Config{
		MaxAttempts: 3,
		BaseDelay:   2 * time.Second,
		MaxDelay:    8 * time.Second,
	}
}

// WithExponentialBackoff executes an operation with exponential backoff retry logic
// It will retry the operation up to MaxAttempts times with exponentially increasing delays
// If all attempts fail, it returns the last error encountered
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

// WithSimpleRetry executes an operation with simple retry logic (no exponential backoff)
// Uses a fixed delay between retries, suitable for operations that don't benefit from exponential backoff
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