package retry

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// RetryableFunc is a generic function type that returns a value and an error.
// It's used for operations that need to return data along with potential errors.
//
// Example:
//
//	fetchData := retry.RetryableFunc[[]byte](func() ([]byte, error) {
//	    return http.Get(url)
//	})
type RetryableFunc[T any] func() (T, error)

// IsRetryable defines an interface for determining if an error should trigger a retry.
// Implement this interface to create custom retry logic based on error types or content.
type IsRetryable interface {
	ShouldRetry(error) bool
}

// RetryPredicate is a function type that implements IsRetryable.
// It provides a convenient way to create retry predicates from simple functions.
type RetryPredicate func(error) bool

// ShouldRetry implements IsRetryable interface for RetryPredicate
func (p RetryPredicate) ShouldRetry(err error) bool {
	return p(err)
}

// AlwaysRetry is a predicate that always returns true for any non-nil error.
// Use this when you want to retry all errors up to the maximum attempt limit.
var AlwaysRetry = RetryPredicate(func(err error) bool {
	return err != nil
})

// NeverRetry is a predicate that always returns false.
// Use this to disable retries while still using the retry framework for consistency.
var NeverRetry = RetryPredicate(func(err error) bool {
	return false
})

// ConfigWithJitter extends Config with jitter support to prevent thundering herd.
// Jitter adds randomization to retry delays, helping to distribute load when multiple
// clients are retrying failed operations simultaneously.
type ConfigWithJitter struct {
	Config
	JitterFactor float64 // Jitter factor (0.0 to 1.0), e.g., 0.2 for ±20% jitter
}

// DefaultConfigWithJitter returns a default retry configuration with 20% jitter.
// This is the recommended configuration for most use cases as it helps prevent
// thundering herd problems in distributed systems.
func DefaultConfigWithJitter() ConfigWithJitter {
	return ConfigWithJitter{
		Config:       DefaultConfig(),
		JitterFactor: 0.2, // ±20% jitter
	}
}

// WithExponentialBackoffJitter executes an operation with exponential backoff and jitter.
// Jitter adds randomization to the retry delay to prevent thundering herd problems.
// The actual delay will be: baseDelay * 2^(attempt-1) ± (jitterFactor * calculatedDelay)
//
// Example:
//
//	cfg := retry.DefaultConfigWithJitter()
//	err := retry.WithExponentialBackoffJitter(ctx, cfg, log, "api_call", func() error {
//	    return callAPI()
//	})
func WithExponentialBackoffJitter(ctx context.Context, cfg ConfigWithJitter, log *logger.Logger, operationName string, operation func() error) error {
	return WithExponentialBackoffJitterPredicate(ctx, cfg, log, operationName, AlwaysRetry, operation)
}

// WithExponentialBackoffJitterPredicate executes an operation with exponential backoff, jitter, and custom retry predicate.
// This provides fine-grained control over which errors should trigger retries.
//
// Example:
//
//	// Only retry on specific errors
//	predicate := retry.RetryPredicate(func(err error) bool {
//	    var tempErr *TemporaryError
//	    return errors.As(err, &tempErr)
//	})
//	
//	err := retry.WithExponentialBackoffJitterPredicate(ctx, cfg, log, "api_call", predicate, operation)
func WithExponentialBackoffJitterPredicate(ctx context.Context, cfg ConfigWithJitter, log *logger.Logger, operationName string, predicate IsRetryable, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Execute the operation
		if err := operation(); err != nil {
			lastErr = err

			// Check if we should retry this error
			if !predicate.ShouldRetry(err) {
				log.Info("Operation failed with non-retryable error",
					"operation", operationName,
					"attempt", attempt,
					"error", err.Error())
				return err
			}

			// If this was the last attempt, log critical error and return
			if attempt == cfg.MaxAttempts {
				log.Critical("Operation failed after all retry attempts",
					"operation", operationName,
					"attempts", cfg.MaxAttempts,
					"error", err.Error())
				return err
			}

			// Calculate base delay using exponential backoff
			baseDelay := time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt-1)))
			if baseDelay > cfg.MaxDelay {
				baseDelay = cfg.MaxDelay
			}

			// Apply jitter
			delay := applyJitter(baseDelay, cfg.JitterFactor)

			log.Warn("Operation failed, retrying with jitter",
				"operation", operationName,
				"attempt", attempt,
				"max_attempts", cfg.MaxAttempts,
				"base_delay", baseDelay.String(),
				"actual_delay", delay.String(),
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

// DoWithResult executes a retryable function that returns a value.
// It handles both the retry logic and the value return, making it easier to retry
// operations that need to return data.
//
// Example:
//
//	data, err := retry.DoWithResult(ctx, cfg, log, "fetch_user", func() (*User, error) {
//	    return fetchUserFromAPI(userID)
//	})
func DoWithResult[T any](ctx context.Context, cfg ConfigWithJitter, log *logger.Logger, operationName string, fn RetryableFunc[T]) (T, error) {
	return DoWithResultPredicate(ctx, cfg, log, operationName, AlwaysRetry, fn)
}

// DoWithResultPredicate executes a retryable function with a custom retry predicate.
// This combines value-returning operations with custom retry logic.
//
// Example:
//
//	// Only retry on network errors
//	predicate := retry.IsNetworkError()
//	data, err := retry.DoWithResultPredicate(ctx, cfg, log, "fetch_data", predicate, fetchFunc)
func DoWithResultPredicate[T any](ctx context.Context, cfg ConfigWithJitter, log *logger.Logger, operationName string, predicate IsRetryable, fn RetryableFunc[T]) (T, error) {
	var lastErr error
	var zero T

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Execute the operation
		result, err := fn()
		if err != nil {
			lastErr = err

			// Check if we should retry this error
			if !predicate.ShouldRetry(err) {
				log.Info("Operation failed with non-retryable error",
					"operation", operationName,
					"attempt", attempt,
					"error", err.Error())
				return zero, err
			}

			// If this was the last attempt, log critical error and return
			if attempt == cfg.MaxAttempts {
				log.Critical("Operation failed after all retry attempts",
					"operation", operationName,
					"attempts", cfg.MaxAttempts,
					"error", err.Error())
				return zero, err
			}

			// Calculate base delay using exponential backoff
			baseDelay := time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt-1)))
			if baseDelay > cfg.MaxDelay {
				baseDelay = cfg.MaxDelay
			}

			// Apply jitter
			delay := applyJitter(baseDelay, cfg.JitterFactor)

			log.Warn("Operation failed, retrying with jitter",
				"operation", operationName,
				"attempt", attempt,
				"max_attempts", cfg.MaxAttempts,
				"base_delay", baseDelay.String(),
				"actual_delay", delay.String(),
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
				return zero, ctx.Err()
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
			return result, nil
		}
	}

	return zero, lastErr
}

// applyJitter adds random jitter to a delay to prevent thundering herd.
// The jitter factor determines the maximum percentage of randomization.
// For example, a jitter factor of 0.2 adds ±20% randomization to the delay.
func applyJitter(delay time.Duration, jitterFactor float64) time.Duration {
	if jitterFactor <= 0 {
		return delay
	}
	
	// Ensure jitter factor is between 0 and 1
	if jitterFactor > 1 {
		jitterFactor = 1
	}

	// Calculate jitter range
	jitterRange := float64(delay) * jitterFactor
	
	// Generate random jitter between -jitterRange and +jitterRange
	jitter := (rand.Float64()*2 - 1) * jitterRange
	
	// Apply jitter to delay
	newDelay := time.Duration(float64(delay) + jitter)
	
	// Ensure delay doesn't go below 0
	if newDelay < 0 {
		newDelay = 0
	}
	
	return newDelay
}

// CombinePredicates creates a predicate that requires all predicates to return true.
// This is useful for creating complex retry conditions that must satisfy multiple criteria.
//
// Example:
//
//	// Retry only on network errors that are not timeout errors
//	predicate := retry.CombinePredicates(
//	    retry.IsNetworkError(),
//	    retry.RetryPredicate(func(err error) bool {
//	        return !errors.Is(err, context.DeadlineExceeded)
//	    }),
//	)
func CombinePredicates(predicates ...IsRetryable) IsRetryable {
	return RetryPredicate(func(err error) bool {
		for _, p := range predicates {
			if !p.ShouldRetry(err) {
				return false
			}
		}
		return true
	})
}

// AnyPredicate creates a predicate that returns true if any predicate returns true.
// This is useful for creating retry conditions where multiple error types should be retried.
//
// Example:
//
//	// Retry on either network errors or specific HTTP status codes
//	predicate := retry.AnyPredicate(
//	    retry.IsNetworkError(),
//	    retry.IsRetryableHTTPStatus(500, 502, 503),
//	)
func AnyPredicate(predicates ...IsRetryable) IsRetryable {
	return RetryPredicate(func(err error) bool {
		for _, p := range predicates {
			if p.ShouldRetry(err) {
				return true
			}
		}
		return false
	})
}