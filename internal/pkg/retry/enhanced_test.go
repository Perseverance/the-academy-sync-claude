package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

func TestWithExponentialBackoffJitter(t *testing.T) {
	ctx := context.Background()
	log := logger.New("test")
	cfg := ConfigWithJitter{
		Config: Config{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    1 * time.Second,
		},
		JitterFactor: 0.2,
	}

	t.Run("jitter is applied", func(t *testing.T) {
		// Run multiple times to verify jitter creates variation
		delays := make([]time.Duration, 5)
		
		for i := 0; i < 5; i++ {
			start := time.Now()
			callCount := 0
			
			operation := func() error {
				callCount++
				if callCount < 2 {
					return errors.New("retry")
				}
				return nil
			}
			
			err := WithExponentialBackoffJitter(ctx, cfg, log, "test_jitter", operation)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			delays[i] = time.Since(start)
		}
		
		// Check that not all delays are identical (jitter creates variation)
		allSame := true
		for i := 1; i < len(delays); i++ {
			// Allow for 10ms tolerance
			if delays[i] < delays[0]-10*time.Millisecond || delays[i] > delays[0]+10*time.Millisecond {
				allSame = false
				break
			}
		}
		
		if allSame {
			t.Error("Expected jitter to create variation in delays, but all delays were similar")
		}
	})
}

func TestDoWithResult(t *testing.T) {
	ctx := context.Background()
	log := logger.New("test")
	cfg := DefaultConfigWithJitter()
	cfg.MaxAttempts = 3
	cfg.BaseDelay = 50 * time.Millisecond

	t.Run("returns value on success", func(t *testing.T) {
		fn := func() (string, error) {
			return "success", nil
		}

		result, err := DoWithResult(ctx, cfg, log, "test_success", fn)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "success" {
			t.Errorf("expected 'success', got %s", result)
		}
	})

	t.Run("retries and returns value", func(t *testing.T) {
		attempt := 0
		fn := func() (int, error) {
			attempt++
			if attempt < 3 {
				return 0, errors.New("retry")
			}
			return 42, nil
		}

		result, err := DoWithResult(ctx, cfg, log, "test_retry", fn)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 42 {
			t.Errorf("expected 42, got %d", result)
		}
		if attempt != 3 {
			t.Errorf("expected 3 attempts, got %d", attempt)
		}
	})

	t.Run("returns zero value on failure", func(t *testing.T) {
		fn := func() (string, error) {
			return "", errors.New("always fail")
		}

		result, err := DoWithResult(ctx, cfg, log, "test_failure", fn)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if result != "" {
			t.Errorf("expected empty string, got %s", result)
		}
	})
}

func TestRetryPredicates(t *testing.T) {
	ctx := context.Background()
	log := logger.New("test")
	cfg := DefaultConfigWithJitter()
	cfg.MaxAttempts = 2
	cfg.BaseDelay = 10 * time.Millisecond

	t.Run("AlwaysRetry retries all errors", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return errors.New("error")
		}

		err := WithExponentialBackoffJitterPredicate(ctx, cfg, log, "test", AlwaysRetry, operation)
		if err == nil {
			t.Error("expected error")
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("NeverRetry never retries", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return errors.New("error")
		}

		err := WithExponentialBackoffJitterPredicate(ctx, cfg, log, "test", NeverRetry, operation)
		if err == nil {
			t.Error("expected error")
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("custom predicate", func(t *testing.T) {
		retryableErr := errors.New("retryable")
		nonRetryableErr := errors.New("non-retryable")
		
		predicate := RetryPredicate(func(err error) bool {
			return errors.Is(err, retryableErr)
		})

		// Test retryable error
		attempts := 0
		operation := func() error {
			attempts++
			return retryableErr
		}

		err := WithExponentialBackoffJitterPredicate(ctx, cfg, log, "test", predicate, operation)
		if err == nil {
			t.Error("expected error")
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts for retryable error, got %d", attempts)
		}

		// Test non-retryable error
		attempts = 0
		operation = func() error {
			attempts++
			return nonRetryableErr
		}

		err = WithExponentialBackoffJitterPredicate(ctx, cfg, log, "test", predicate, operation)
		if err == nil {
			t.Error("expected error")
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
		}
	})
}

func TestCombinePredicates(t *testing.T) {
	err1 := errors.New("error1")
	err2 := errors.New("error2")
	err3 := errors.New("error3")

	pred1 := RetryPredicate(func(err error) bool {
		return errors.Is(err, err1) || errors.Is(err, err2)
	})

	pred2 := RetryPredicate(func(err error) bool {
		return errors.Is(err, err2) || errors.Is(err, err3)
	})

	combined := CombinePredicates(pred1, pred2)

	// Only err2 should pass both predicates
	if !combined.ShouldRetry(err2) {
		t.Error("expected err2 to be retryable")
	}
	if combined.ShouldRetry(err1) {
		t.Error("expected err1 to not be retryable")
	}
	if combined.ShouldRetry(err3) {
		t.Error("expected err3 to not be retryable")
	}
}

func TestAnyPredicate(t *testing.T) {
	err1 := errors.New("error1")
	err2 := errors.New("error2")
	err3 := errors.New("error3")
	err4 := errors.New("error4")

	pred1 := RetryPredicate(func(err error) bool {
		return errors.Is(err, err1) || errors.Is(err, err2)
	})

	pred2 := RetryPredicate(func(err error) bool {
		return errors.Is(err, err2) || errors.Is(err, err3)
	})

	any := AnyPredicate(pred1, pred2)

	// err1, err2, and err3 should all be retryable
	if !any.ShouldRetry(err1) {
		t.Error("expected err1 to be retryable")
	}
	if !any.ShouldRetry(err2) {
		t.Error("expected err2 to be retryable")
	}
	if !any.ShouldRetry(err3) {
		t.Error("expected err3 to be retryable")
	}
	if any.ShouldRetry(err4) {
		t.Error("expected err4 to not be retryable")
	}
}

func TestApplyJitter(t *testing.T) {
	baseDelay := 100 * time.Millisecond

	t.Run("no jitter", func(t *testing.T) {
		result := applyJitter(baseDelay, 0)
		if result != baseDelay {
			t.Errorf("expected %v, got %v", baseDelay, result)
		}
	})

	t.Run("with jitter", func(t *testing.T) {
		// Test multiple times to ensure randomness
		minDelay := time.Duration(float64(baseDelay) * 0.8)
		maxDelay := time.Duration(float64(baseDelay) * 1.2)
		
		inRange := 0
		for i := 0; i < 100; i++ {
			result := applyJitter(baseDelay, 0.2)
			if result >= minDelay && result <= maxDelay {
				inRange++
			}
		}
		
		// At least 95% should be in range (allowing for randomness)
		if inRange < 95 {
			t.Errorf("expected at least 95%% of results in range, got %d%%", inRange)
		}
	})

	t.Run("capped jitter factor", func(t *testing.T) {
		result := applyJitter(baseDelay, 2.0) // Should be capped at 1.0
		// Result should be between 0 and 2*baseDelay
		if result < 0 || result > 2*baseDelay {
			t.Errorf("jitter result out of expected range: %v", result)
		}
	})
}

func BenchmarkDoWithResult(b *testing.B) {
	ctx := context.Background()
	log := logger.New("bench")
	cfg := DefaultConfigWithJitter()
	cfg.MaxAttempts = 1

	fn := func() (string, error) {
		return "result", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DoWithResult(ctx, cfg, log, "bench", fn)
	}
}

func ExampleDoWithResult() {
	ctx := context.Background()
	log := logger.New("example")
	cfg := DefaultConfigWithJitter()

	// Example: Fetch data with retry
	fetchData := func() ([]byte, error) {
		// Simulate API call
		return []byte("data"), nil
	}

	data, err := DoWithResult(ctx, cfg, log, "fetch_data", fetchData)
	if err != nil {
		fmt.Printf("Failed to fetch data: %v\n", err)
		return
	}
	
	fmt.Printf("Fetched %d bytes\n", len(data))
	// Output: Fetched 4 bytes
}