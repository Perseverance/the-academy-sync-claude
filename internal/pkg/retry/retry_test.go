package retry

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// mockClock allows us to control time in tests
type mockClock struct {
	mu       sync.Mutex
	current  time.Time
	sleepers []chan time.Time
}

func newMockClock(start time.Time) *mockClock {
	return &mockClock{
		current: start,
	}
}

func (m *mockClock) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

func (m *mockClock) After(d time.Duration) <-chan time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	ch := make(chan time.Time, 1)
	m.sleepers = append(m.sleepers, ch)
	
	// Simulate immediate advancement for testing
	go func() {
		time.Sleep(1 * time.Millisecond) // Small real delay to allow goroutine switching
		m.mu.Lock()
		m.current = m.current.Add(d)
		m.mu.Unlock()
		ch <- m.current
	}()
	
	return ch
}

func (m *mockClock) advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = m.current.Add(d)
	
	// Wake up all sleepers
	for _, ch := range m.sleepers {
		select {
		case ch <- m.current:
		default:
		}
	}
	m.sleepers = nil
}

func TestWithExponentialBackoff_Success(t *testing.T) {
	tests := []struct {
		name           string
		failureCount   int
		expectedCalls  int
		expectedResult error
	}{
		{
			name:           "success on first attempt",
			failureCount:   0,
			expectedCalls:  1,
			expectedResult: nil,
		},
		{
			name:           "success on second attempt",
			failureCount:   1,
			expectedCalls:  2,
			expectedResult: nil,
		},
		{
			name:           "success on third attempt",
			failureCount:   2,
			expectedCalls:  3,
			expectedResult: nil,
		},
		{
			name:           "all attempts fail",
			failureCount:   3,
			expectedCalls:  3,
			expectedResult: errors.New("operation failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			log := logger.New("test")
			cfg := Config{
				MaxAttempts: 3,
				BaseDelay:   100 * time.Millisecond,
				MaxDelay:    1 * time.Second,
			}

			callCount := 0
			operation := func() error {
				callCount++
				if callCount <= tt.failureCount {
					return errors.New("operation failed")
				}
				return nil
			}

			err := WithExponentialBackoff(ctx, cfg, log, "test_operation", operation)

			if callCount != tt.expectedCalls {
				t.Errorf("expected %d calls, got %d", tt.expectedCalls, callCount)
			}

			if tt.expectedResult == nil && err != nil {
				t.Errorf("expected success, got error: %v", err)
			} else if tt.expectedResult != nil && err == nil {
				t.Errorf("expected error, got success")
			}
		})
	}
}

func TestWithExponentialBackoff_DelayCalculation(t *testing.T) {
	ctx := context.Background()
	log := logger.New("test")
	cfg := Config{
		MaxAttempts: 4,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    500 * time.Millisecond,
	}

	delays := []time.Duration{}
	lastCallTime := time.Now()
	callCount := 0

	operation := func() error {
		callCount++
		now := time.Now()
		if callCount > 1 {
			delay := now.Sub(lastCallTime)
			delays = append(delays, delay)
		}
		lastCallTime = now
		
		if callCount < 4 {
			return errors.New("retry me")
		}
		return nil
	}

	err := WithExponentialBackoff(ctx, cfg, log, "test_delays", operation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify delays with some tolerance for timing
	expectedDelays := []time.Duration{
		100 * time.Millisecond, // 100ms * 2^0
		200 * time.Millisecond, // 100ms * 2^1
		400 * time.Millisecond, // 100ms * 2^2
	}

	if len(delays) != len(expectedDelays) {
		t.Fatalf("expected %d delays, got %d", len(expectedDelays), len(delays))
	}

	for i, expected := range expectedDelays {
		// Allow 50ms tolerance for timing variations
		if delays[i] < expected-50*time.Millisecond || delays[i] > expected+50*time.Millisecond {
			t.Errorf("delay %d: expected ~%v, got %v", i+1, expected, delays[i])
		}
	}
}

func TestWithExponentialBackoff_MaxDelay(t *testing.T) {
	ctx := context.Background()
	log := logger.New("test")
	cfg := Config{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    200 * time.Millisecond, // Low max delay to test capping
	}

	delays := []time.Duration{}
	lastCallTime := time.Now()
	callCount := 0

	operation := func() error {
		callCount++
		now := time.Now()
		if callCount > 1 {
			delay := now.Sub(lastCallTime)
			delays = append(delays, delay)
		}
		lastCallTime = now
		return errors.New("always fail")
	}

	_ = WithExponentialBackoff(ctx, cfg, log, "test_max_delay", operation)

	// Verify that delays are capped at MaxDelay
	for i, delay := range delays {
		if delay > cfg.MaxDelay+50*time.Millisecond {
			t.Errorf("delay %d exceeded max delay: got %v, max %v", i+1, delay, cfg.MaxDelay)
		}
	}
}

func TestWithExponentialBackoff_ContextCancellation(t *testing.T) {
	log := logger.New("test")
	cfg := Config{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second, // Long delay to ensure context cancels first
		MaxDelay:    10 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	operation := func() error {
		callCount++
		if callCount == 1 {
			// Cancel context after first attempt
			go func() {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}()
		}
		return errors.New("operation failed")
	}

	err := WithExponentialBackoff(ctx, cfg, log, "test_cancellation", operation)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 call before cancellation, got %d", callCount)
	}
}

func TestWithSimpleRetry_Success(t *testing.T) {
	tests := []struct {
		name           string
		maxAttempts    int
		failureCount   int
		expectedCalls  int
		expectedResult error
	}{
		{
			name:           "success on first attempt",
			maxAttempts:    3,
			failureCount:   0,
			expectedCalls:  1,
			expectedResult: nil,
		},
		{
			name:           "success on last attempt",
			maxAttempts:    3,
			failureCount:   2,
			expectedCalls:  3,
			expectedResult: nil,
		},
		{
			name:           "all attempts fail",
			maxAttempts:    2,
			failureCount:   2,
			expectedCalls:  2,
			expectedResult: errors.New("operation failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			log := logger.New("test")
			delay := 50 * time.Millisecond

			callCount := 0
			operation := func() error {
				callCount++
				if callCount <= tt.failureCount {
					return errors.New("operation failed")
				}
				return nil
			}

			err := WithSimpleRetry(ctx, tt.maxAttempts, delay, log, "test_operation", operation)

			if callCount != tt.expectedCalls {
				t.Errorf("expected %d calls, got %d", tt.expectedCalls, callCount)
			}

			if tt.expectedResult == nil && err != nil {
				t.Errorf("expected success, got error: %v", err)
			} else if tt.expectedResult != nil && err == nil {
				t.Errorf("expected error, got success")
			}
		})
	}
}

func TestWithSimpleRetry_FixedDelay(t *testing.T) {
	ctx := context.Background()
	log := logger.New("test")
	delay := 100 * time.Millisecond

	delays := []time.Duration{}
	lastCallTime := time.Now()
	callCount := 0

	operation := func() error {
		callCount++
		now := time.Now()
		if callCount > 1 {
			actualDelay := now.Sub(lastCallTime)
			delays = append(delays, actualDelay)
		}
		lastCallTime = now
		
		if callCount < 3 {
			return errors.New("retry me")
		}
		return nil
	}

	err := WithSimpleRetry(ctx, 3, delay, log, "test_fixed_delay", operation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all delays are roughly the same
	for i, actualDelay := range delays {
		if actualDelay < delay-50*time.Millisecond || actualDelay > delay+50*time.Millisecond {
			t.Errorf("delay %d: expected ~%v, got %v", i+1, delay, actualDelay)
		}
	}
}

func TestWithSimpleRetry_ContextCancellation(t *testing.T) {
	log := logger.New("test")
	delay := 1 * time.Second // Long delay to ensure context cancels first

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	operation := func() error {
		callCount++
		if callCount == 1 {
			// Cancel context after first attempt
			go func() {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}()
		}
		return errors.New("operation failed")
	}

	err := WithSimpleRetry(ctx, 3, delay, log, "test_cancellation", operation)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 call before cancellation, got %d", callCount)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	if cfg.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", cfg.MaxAttempts)
	}
	if cfg.BaseDelay != 1*time.Second {
		t.Errorf("expected BaseDelay=1s, got %v", cfg.BaseDelay)
	}
	if cfg.MaxDelay != 10*time.Second {
		t.Errorf("expected MaxDelay=10s, got %v", cfg.MaxDelay)
	}
}

func TestCriticalConfig(t *testing.T) {
	cfg := CriticalConfig()
	
	if cfg.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", cfg.MaxAttempts)
	}
	if cfg.BaseDelay != 2*time.Second {
		t.Errorf("expected BaseDelay=2s, got %v", cfg.BaseDelay)
	}
	if cfg.MaxDelay != 8*time.Second {
		t.Errorf("expected MaxDelay=8s, got %v", cfg.MaxDelay)
	}
}

func TestConcurrentRetries(t *testing.T) {
	ctx := context.Background()
	log := logger.New("test")
	cfg := DefaultConfig()
	cfg.MaxAttempts = 2
	cfg.BaseDelay = 10 * time.Millisecond

	// Test that multiple retry operations can run concurrently
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	successCount := int32(0)
	totalCalls := int32(0)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			callCount := 0
			operation := func() error {
				atomic.AddInt32(&totalCalls, 1)
				callCount++
				if callCount < 2 {
					return fmt.Errorf("goroutine %d: attempt %d failed", id, callCount)
				}
				return nil
			}

			err := WithExponentialBackoff(ctx, cfg, log, fmt.Sprintf("concurrent_%d", id), operation)
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if successCount != numGoroutines {
		t.Errorf("expected all %d goroutines to succeed, got %d", numGoroutines, successCount)
	}

	// Each goroutine should make 2 calls
	expectedCalls := int32(numGoroutines * 2)
	if totalCalls != expectedCalls {
		t.Errorf("expected %d total calls, got %d", expectedCalls, totalCalls)
	}
}

func TestRetryWithPanic(t *testing.T) {
	ctx := context.Background()
	log := logger.New("test")
	cfg := DefaultConfig()
	cfg.MaxAttempts = 2

	callCount := 0
	operation := func() error {
		callCount++
		if callCount == 1 {
			panic("test panic")
		}
		return nil
	}

	// The retry functions don't handle panics, so we expect them to propagate
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to propagate")
		} else if r != "test panic" {
			t.Errorf("expected 'test panic', got %v", r)
		}
	}()

	_ = WithExponentialBackoff(ctx, cfg, log, "test_panic", operation)
}

// Benchmark tests
func BenchmarkWithExponentialBackoff_Success(b *testing.B) {
	ctx := context.Background()
	log := logger.New("bench")
	cfg := Config{
		MaxAttempts: 1,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	}

	operation := func() error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WithExponentialBackoff(ctx, cfg, log, "bench_operation", operation)
	}
}

func BenchmarkWithExponentialBackoff_WithRetries(b *testing.B) {
	ctx := context.Background()
	log := logger.New("bench")
	cfg := Config{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callCount := 0
		operation := func() error {
			callCount++
			if callCount < 2 {
				return errors.New("retry")
			}
			return nil
		}
		_ = WithExponentialBackoff(ctx, cfg, log, "bench_operation", operation)
	}
}