package retry

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

func TestHTTPRetryPredicate(t *testing.T) {
	config := DefaultHTTPConfig()
	predicate := NewHTTPRetryPredicate(config)

	tests := []struct {
		name        string
		err         error
		shouldRetry bool
	}{
		{
			name:        "nil error",
			err:         nil,
			shouldRetry: false,
		},
		{
			name:        "429 status",
			err:         &HTTPError{StatusCode: 429},
			shouldRetry: true,
		},
		{
			name:        "502 status",
			err:         &HTTPError{StatusCode: 502},
			shouldRetry: true,
		},
		{
			name:        "503 status",
			err:         &HTTPError{StatusCode: 503},
			shouldRetry: true,
		},
		{
			name:        "504 status",
			err:         &HTTPError{StatusCode: 504},
			shouldRetry: true,
		},
		{
			name:        "400 status",
			err:         &HTTPError{StatusCode: 400},
			shouldRetry: false,
		},
		{
			name:        "401 status",
			err:         &HTTPError{StatusCode: 401},
			shouldRetry: false,
		},
		{
			name:        "404 status",
			err:         &HTTPError{StatusCode: 404},
			shouldRetry: false,
		},
		{
			name:        "network timeout",
			err:         &net.OpError{Op: "dial", Err: &timeoutError{}},
			shouldRetry: true,
		},
		{
			name:        "connection refused",
			err:         &net.OpError{Op: "dial", Err: fmt.Errorf("connection refused")},
			shouldRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := predicate.ShouldRetry(tt.err)
			if result != tt.shouldRetry {
				t.Errorf("expected ShouldRetry=%v, got %v", tt.shouldRetry, result)
			}
		})
	}
}

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

func TestWithHTTPRetry(t *testing.T) {
	ctx := context.Background()
	log := logger.New("test")
	config := DefaultHTTPConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 10 * time.Millisecond

	t.Run("success on first attempt", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		client := server.Client()
		req, _ := http.NewRequest("GET", server.URL, nil)

		resp, err := WithHTTPRetry(ctx, client, req, config, log)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("retry on 503", func(t *testing.T) {
		attempts := int32(0)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempt := atomic.AddInt32(&attempts, 1)
			if attempt < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("service unavailable"))
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			}
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		client := server.Client()
		req, _ := http.NewRequest("GET", server.URL, nil)

		resp, err := WithHTTPRetry(ctx, client, req, config, log)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
		if atomic.LoadInt32(&attempts) != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("no retry on 404", func(t *testing.T) {
		attempts := int32(0)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		client := server.Client()
		req, _ := http.NewRequest("GET", server.URL, nil)

		resp, err := WithHTTPRetry(ctx, client, req, config, log)
		if err == nil {
			t.Error("expected HTTPError")
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
		if atomic.LoadInt32(&attempts) != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("retry with body", func(t *testing.T) {
		attempts := int32(0)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if string(body) != "test body" {
				t.Error("request body not preserved")
			}
			
			attempt := atomic.AddInt32(&attempts, 1)
			if attempt < 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		client := server.Client()
		req, _ := http.NewRequest("POST", server.URL, strings.NewReader("test body"))

		resp, err := WithHTTPRetry(ctx, client, req, config, log)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if atomic.LoadInt32(&attempts) != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})
}

func TestRetryAfterHeader(t *testing.T) {
	ctx := context.Background()
	log := logger.New("test")
	config := DefaultHTTPConfig()
	config.MaxAttempts = 2
	config.RespectRetryAfter = true
	config.MaxRetryAfter = 5 * time.Second

	t.Run("respect Retry-After seconds", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		client := server.Client()
		req, _ := http.NewRequest("GET", server.URL, nil)

		start := time.Now()
		_, err := WithHTTPRetry(ctx, client, req, config, log)
		duration := time.Since(start)

		if err == nil {
			t.Error("expected error")
		}
		// Should wait approximately 1 second
		if duration < 900*time.Millisecond || duration > 1200*time.Millisecond {
			t.Errorf("expected ~1s delay, got %v", duration)
		}
	})

	t.Run("ignore excessive Retry-After", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "60") // 60 seconds, exceeds MaxRetryAfter
			w.WriteHeader(http.StatusTooManyRequests)
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		client := server.Client()
		req, _ := http.NewRequest("GET", server.URL, nil)

		start := time.Now()
		_, err := WithHTTPRetry(ctx, client, req, config, log)
		duration := time.Since(start)

		if err == nil {
			t.Error("expected error")
		}
		// Should use regular backoff, not 60 seconds
		if duration > 2*time.Second {
			t.Errorf("expected shorter delay, got %v", duration)
		}
	})
}

func TestHTTPRetryTransport(t *testing.T) {
	attempts := int32(0)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	log := logger.New("test")
	config := DefaultHTTPConfig()
	config.MaxAttempts = 3
	config.BaseDelay = 10 * time.Millisecond

	client := NewHTTPClientWithRetry(config, log)
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestIsNetworkError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "timeout error",
			err:      &net.OpError{Op: "dial", Err: &timeoutError{}},
			expected: true,
		},
		{
			name:     "connection refused",
			err:      &net.OpError{Op: "dial", Err: fmt.Errorf("connection refused")},
			expected: true,
		},
		{
			name:     "DNS error",
			err:      &net.DNSError{Name: "example.com", Err: "no such host"},
			expected: true,
		},
		{
			name:     "EOF error",
			err:      io.EOF,
			expected: true,
		},
		{
			name:     "broken pipe",
			err:      fmt.Errorf("write: broken pipe"),
			expected: true,
		},
		{
			name:     "regular error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNetworkError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHTTPOrNetworkError(t *testing.T) {
	predicate := HTTPOrNetworkError(500, 502)

	tests := []struct {
		name        string
		err         error
		shouldRetry bool
	}{
		{
			name:        "HTTP 500",
			err:         &HTTPError{StatusCode: 500},
			shouldRetry: true,
		},
		{
			name:        "HTTP 502",
			err:         &HTTPError{StatusCode: 502},
			shouldRetry: true,
		},
		{
			name:        "HTTP 404",
			err:         &HTTPError{StatusCode: 404},
			shouldRetry: false,
		},
		{
			name:        "network timeout",
			err:         &net.OpError{Op: "dial", Err: &timeoutError{}},
			shouldRetry: true,
		},
		{
			name:        "regular error",
			err:         fmt.Errorf("some error"),
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := predicate.ShouldRetry(tt.err)
			if result != tt.shouldRetry {
				t.Errorf("expected %v, got %v", tt.shouldRetry, result)
			}
		})
	}
}

func ExampleNewHTTPClientWithRetry() {
	log := logger.New("example")
	config := DefaultHTTPConfig()
	config.MaxAttempts = 3
	
	// Create HTTP client with automatic retry
	client := NewHTTPClientWithRetry(config, log)
	
	// Use the client normally - retries happen automatically
	resp, err := client.Get("https://api.example.com/data")
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	fmt.Printf("Request succeeded with status: %d\n", resp.StatusCode)
}