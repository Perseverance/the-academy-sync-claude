package retry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// HTTPConfig extends ConfigWithJitter with HTTP-specific settings.
// It provides specialized configuration for retrying HTTP requests, including
// status code handling and Retry-After header support.
type HTTPConfig struct {
	ConfigWithJitter
	RetryableStatusCodes []int           // HTTP status codes that should trigger a retry
	RespectRetryAfter    bool            // Whether to respect Retry-After headers
	MaxRetryAfter        time.Duration   // Maximum time to wait for Retry-After
}

// DefaultHTTPConfig returns a default HTTP retry configuration.
// It retries on common transient errors:
// - 429 (Too Many Requests)
// - 502 (Bad Gateway)
// - 503 (Service Unavailable)
// - 504 (Gateway Timeout)
// It also respects Retry-After headers up to 30 seconds.
func DefaultHTTPConfig() HTTPConfig {
	return HTTPConfig{
		ConfigWithJitter: DefaultConfigWithJitter(),
		RetryableStatusCodes: []int{
			http.StatusTooManyRequests,     // 429
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout,      // 504
		},
		RespectRetryAfter: true,
		MaxRetryAfter:     30 * time.Second,
	}
}

// HTTPRetryPredicate is a predicate that checks if an HTTP error should be retried.
// It considers both HTTP status codes and network-level errors.
type HTTPRetryPredicate struct {
	config HTTPConfig
}

// NewHTTPRetryPredicate creates a new HTTP retry predicate with the given configuration.
// The predicate will retry based on the status codes defined in the configuration
// and will always retry network-level errors.
func NewHTTPRetryPredicate(config HTTPConfig) *HTTPRetryPredicate {
	return &HTTPRetryPredicate{config: config}
}

// ShouldRetry determines if an HTTP error should trigger a retry.
// It returns true for:
// - Network errors (timeouts, connection refused, etc.)
// - HTTP status codes configured as retryable
// - nil errors always return false
func (p *HTTPRetryPredicate) ShouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Check for network errors
	if isNetworkError(err) {
		return true
	}

	// Check for HTTP response errors
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return p.shouldRetryStatusCode(httpErr.StatusCode)
	}

	return false
}

// shouldRetryStatusCode checks if a status code is retryable
func (p *HTTPRetryPredicate) shouldRetryStatusCode(statusCode int) bool {
	for _, code := range p.config.RetryableStatusCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// HTTPError represents an HTTP error with status code.
// It's returned when an HTTP request completes but with a non-success status code.
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Status)
}

// WithHTTPRetry performs an HTTP request with retry logic.
// It handles:
// - Automatic request body preservation across retries
// - Retry-After header parsing and respect
// - Exponential backoff with jitter
// - Network and HTTP error classification
//
// The function returns the last HTTP response even on error, allowing
// callers to inspect headers and body for additional error details.
//
// Example:
//
//	req, _ := http.NewRequest("GET", "https://api.example.com/data", nil)
//	resp, err := retry.WithHTTPRetry(ctx, client, req, config, logger)
//	if err != nil {
//	    // Handle error - resp may still contain useful information
//	}
//	defer resp.Body.Close()
func WithHTTPRetry(ctx context.Context, client *http.Client, req *http.Request, config HTTPConfig, log *logger.Logger) (*http.Response, error) {
	operationName := fmt.Sprintf("HTTP %s %s", req.Method, req.URL.Path)
	predicate := NewHTTPRetryPredicate(config)

	// Clone the request body if present
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		req.Body.Close()
	}

	var lastResp *http.Response
	var lastErr error

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Clone the request for this attempt
		clonedReq := req.Clone(ctx)
		if bodyBytes != nil {
			clonedReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Perform the request
		resp, err := client.Do(clonedReq)
		
		if err != nil {
			lastErr = err
			
			// Check if this is a retryable error
			if !predicate.ShouldRetry(err) {
				log.Info("HTTP request failed with non-retryable error",
					"operation", operationName,
					"attempt", attempt,
					"error", err.Error())
				return nil, err
			}

			// If this was the last attempt, return the error
			if attempt == config.MaxAttempts {
				log.Critical("HTTP request failed after all retry attempts",
					"operation", operationName,
					"attempts", config.MaxAttempts,
					"error", err.Error())
				return nil, err
			}

			// Calculate retry delay
			delay := calculateHTTPRetryDelay(attempt, config, nil, log, operationName)

			// Wait before retry
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				log.Error("HTTP retry operation cancelled due to context timeout",
					"operation", operationName,
					"attempt", attempt,
					"context_error", ctx.Err().Error())
				return nil, ctx.Err()
			}
		}

		// Check response status code
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Success
			if attempt > 1 {
				log.Info("HTTP request succeeded after retry",
					"operation", operationName,
					"attempt", attempt,
					"status_code", resp.StatusCode)
			}
			return resp, nil
		}

		// Read response body for error details
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		httpErr := &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(bodyBytes),
		}
		lastErr = httpErr
		lastResp = resp

		// Check if status code is retryable
		if !predicate.shouldRetryStatusCode(resp.StatusCode) {
			log.Info("HTTP request failed with non-retryable status code",
				"operation", operationName,
				"attempt", attempt,
				"status_code", resp.StatusCode,
				"status", resp.Status)
			return resp, httpErr
		}

		// If this was the last attempt, return the error
		if attempt == config.MaxAttempts {
			log.Critical("HTTP request failed after all retry attempts",
				"operation", operationName,
				"attempts", config.MaxAttempts,
				"status_code", resp.StatusCode,
				"status", resp.Status)
			return resp, httpErr
		}

		// Calculate retry delay, considering Retry-After header
		delay := calculateHTTPRetryDelay(attempt, config, resp, log, operationName)

		// Wait before retry
		select {
		case <-time.After(delay):
			continue
		case <-ctx.Done():
			log.Error("HTTP retry operation cancelled due to context timeout",
				"operation", operationName,
				"attempt", attempt,
				"context_error", ctx.Err().Error())
			return resp, ctx.Err()
		}
	}

	return lastResp, lastErr
}

// HTTPRetryTransport is an http.RoundTripper that adds retry logic.
// It can be used as a Transport in http.Client to automatically retry
// all requests made through that client.
//
// Example:
//
//	transport := &retry.HTTPRetryTransport{
//	    Base:   http.DefaultTransport,
//	    Config: retry.DefaultHTTPConfig(),
//	    Logger: logger,
//	}
//	client := &http.Client{Transport: transport}
type HTTPRetryTransport struct {
	Base   http.RoundTripper
	Config HTTPConfig
	Logger *logger.Logger
}

// RoundTrip implements http.RoundTripper with retry logic.
// It wraps the base transport's RoundTrip method with automatic retry capability.
func (t *HTTPRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a client with the base transport
	client := &http.Client{
		Transport: t.Base,
	}

	return WithHTTPRetry(req.Context(), client, req, t.Config, t.Logger)
}

// NewHTTPClientWithRetry creates an HTTP client with built-in retry logic.
// This is the simplest way to add retry capability to HTTP requests.
//
// Example:
//
//	client := retry.NewHTTPClientWithRetry(retry.DefaultHTTPConfig(), logger)
//	resp, err := client.Get("https://api.example.com/data")
//	// Retries happen automatically based on configuration
func NewHTTPClientWithRetry(config HTTPConfig, logger *logger.Logger) *http.Client {
	return &http.Client{
		Transport: &HTTPRetryTransport{
			Base:   http.DefaultTransport,
			Config: config,
			Logger: logger,
		},
	}
}

// calculateHTTPRetryDelay calculates the retry delay for HTTP requests.
// It considers:
// - Exponential backoff with jitter
// - Retry-After headers (if configured)
// - Maximum delay limits
// Priority is given to Retry-After headers when present and within limits.
func calculateHTTPRetryDelay(attempt int, config HTTPConfig, resp *http.Response, log *logger.Logger, operationName string) time.Duration {
	// Calculate base delay using exponential backoff (2^(attempt-1) * baseDelay)
	baseDelay := time.Duration(float64(config.BaseDelay) * math.Pow(2, float64(attempt-1)))
	if baseDelay > config.MaxDelay {
		baseDelay = config.MaxDelay
	}

	// Check for Retry-After header if configured
	if config.RespectRetryAfter && resp != nil {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			// Try to parse as seconds
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				retryDelay := time.Duration(seconds) * time.Second
				if retryDelay <= config.MaxRetryAfter {
					log.Debug("Using Retry-After header for delay",
						"operation", operationName,
						"retry_after", retryDelay.String())
					return retryDelay
				}
				log.Warn("Retry-After header exceeds maximum, using calculated delay",
					"operation", operationName,
					"retry_after", retryDelay.String(),
					"max_retry_after", config.MaxRetryAfter.String())
			} else {
				// Try to parse as HTTP date
				if t, err := http.ParseTime(retryAfter); err == nil {
					retryDelay := time.Until(t)
					if retryDelay > 0 && retryDelay <= config.MaxRetryAfter {
						log.Debug("Using Retry-After header (date) for delay",
							"operation", operationName,
							"retry_after", retryDelay.String())
						return retryDelay
					}
				}
			}
		}
	}

	// Apply jitter to base delay
	return applyJitter(baseDelay, config.JitterFactor)
}

// isNetworkError checks if an error is a network-related error.
// It detects:
// - Timeout errors
// - Connection refused/reset
// - DNS errors
// - Common network error patterns in error messages
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check for timeout errors
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check for connection refused
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// Check for DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Check for specific syscall errors
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}

	// Check for common network error patterns in error message
	errStr := strings.ToLower(err.Error())
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"no such host",
		"timeout",
		"temporary failure",
		"eof",
		"broken pipe",
	}

	for _, pattern := range networkErrors {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// IsRetryableHTTPStatus returns a predicate that checks for specific HTTP status codes.
// Use this to create custom retry logic based on HTTP status codes.
//
// Example:
//
//	// Retry on server errors
//	predicate := retry.IsRetryableHTTPStatus(500, 502, 503, 504)
func IsRetryableHTTPStatus(codes ...int) IsRetryable {
	return RetryPredicate(func(err error) bool {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			for _, code := range codes {
				if httpErr.StatusCode == code {
					return true
				}
			}
		}
		return false
	})
}

// IsNetworkError returns a predicate that checks for network errors.
// This includes timeouts, connection errors, DNS failures, and other
// network-level issues that are typically transient.
func IsNetworkError() IsRetryable {
	return RetryPredicate(func(err error) bool {
		return isNetworkError(err)
	})
}

// HTTPOrNetworkError combines HTTP status code and network error predicates.
// This is a convenient predicate for retrying both specific HTTP errors
// and any network-level errors.
//
// Example:
//
//	// Retry on 500, 502 errors or any network error
//	predicate := retry.HTTPOrNetworkError(500, 502)
func HTTPOrNetworkError(codes ...int) IsRetryable {
	return AnyPredicate(
		IsRetryableHTTPStatus(codes...),
		IsNetworkError(),
	)
}