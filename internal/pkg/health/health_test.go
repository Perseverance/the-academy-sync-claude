package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

func TestCheckSendGrid(t *testing.T) {
	log := logger.New("test")
	checker := NewHealthChecker(log)

	tests := []struct {
		name           string
		apiKey         string
		serverResponse int
		expectHealthy  bool
		errorContains  string
	}{
		{
			name:           "successful health check",
			apiKey:         "valid-key",
			serverResponse: http.StatusOK,
			expectHealthy:  true,
		},
		{
			name:           "unauthorized - invalid API key",
			apiKey:         "invalid-key",
			serverResponse: http.StatusUnauthorized,
			expectHealthy:  false,
			errorContains:  "invalid SendGrid API key",
		},
		{
			name:           "server error",
			apiKey:         "valid-key",
			serverResponse: http.StatusInternalServerError,
			expectHealthy:  false,
			errorContains:  "SendGrid API returned error status: 500",
		},
		{
			name:           "bad request",
			apiKey:         "valid-key",
			serverResponse: http.StatusBadRequest,
			expectHealthy:  false,
			errorContains:  "SendGrid API returned error status: 400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.URL.Path != "/v3/user/profile" {
					t.Errorf("Expected path /v3/user/profile, got %s", r.URL.Path)
				}
				if r.Method != "GET" {
					t.Errorf("Expected GET method, got %s", r.Method)
				}
				
				authHeader := r.Header.Get("Authorization")
				expectedAuth := "Bearer " + tt.apiKey
				if authHeader != expectedAuth {
					t.Errorf("Expected Authorization header '%s', got '%s'", expectedAuth, authHeader)
				}

				// Send response
				w.WriteHeader(tt.serverResponse)
				if tt.serverResponse == http.StatusOK {
					w.Write([]byte(`{"username":"test@example.com"}`))
				}
			}))
			defer server.Close()

			// Override SendGrid API URL for testing
			checker.sendGridAPIURL = server.URL

			ctx := context.Background()
			result := checker.CheckSendGrid(ctx, tt.apiKey)

			if tt.expectHealthy && !result.IsHealthy() {
				t.Errorf("Expected healthy status, got unhealthy: %v", result.Error)
			}

			if !tt.expectHealthy && result.IsHealthy() {
				t.Error("Expected unhealthy status, got healthy")
			}

			if tt.errorContains != "" && result.Error != nil {
				if !strings.Contains(result.Error.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorContains, result.Error.Error())
				}
			}

			if result.Latency == 0 {
				t.Error("Expected non-zero latency")
			}
		})
	}
}

func TestCheckSendGridTimeout(t *testing.T) {
	log := logger.New("test")
	checker := NewHealthChecker(log)

	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Override SendGrid API URL to use test server
	checker.sendGridAPIURL = server.URL

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := checker.CheckSendGrid(ctx, "test-key")

	if result.IsHealthy() {
		t.Error("Expected unhealthy status due to timeout")
	}

	if result.Error == nil {
		t.Error("Expected error for timeout")
	}

	// Verify the error is a timeout error
	if result.Error != nil && !strings.Contains(result.Error.Error(), "context deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", result.Error)
	}
}

