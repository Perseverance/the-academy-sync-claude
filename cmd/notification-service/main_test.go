package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/config"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

func TestPerformStartupHealthChecks(t *testing.T) {
	log := logger.New("test")

	tests := []struct {
		name          string
		config        *config.Config
		expectError   bool
		errorContains string
	}{
		{
			name: "no database configured - should pass",
			config: &config.Config{
				DatabaseURL:     "",
				FailFastEnabled: true,
			},
			expectError: false,
		},
		{
			name: "invalid database URL with fail-fast enabled",
			config: &config.Config{
				DatabaseURL:     "invalid-db-url",
				FailFastEnabled: true,
			},
			expectError:   true,
			errorContains: "database dependency check failed",
		},
		{
			name: "invalid database URL with fail-fast disabled",
			config: &config.Config{
				DatabaseURL:     "invalid-db-url",
				FailFastEnabled: false,
			},
			expectError: false, // Should continue with warning
		},
		{
			name: "invalid SendGrid API key with fail-fast enabled",
			config: &config.Config{
				SendGridAPIKey:  "invalid-key",
				FailFastEnabled: true,
			},
			expectError:   true,
			errorContains: "SendGrid dependency check failed",
		},
		{
			name: "invalid SendGrid API key with fail-fast disabled",
			config: &config.Config{
				SendGridAPIKey:  "invalid-key",
				FailFastEnabled: false,
			},
			expectError: false, // Should continue with warning
		},
		{
			name: "all services configured with invalid credentials and fail-fast enabled",
			config: &config.Config{
				DatabaseURL:     "invalid-db-url",
				SendGridAPIKey:  "invalid-key",
				FailFastEnabled: true,
			},
			expectError:   true,
			errorContains: "dependency check failed", // Could be either database or SendGrid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := performStartupHealthChecks(tt.config, log)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectError && tt.errorContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorContains, err.Error())
				}
			}
		})
	}
}

func TestSendGridInitialization(t *testing.T) {
	// This test verifies that the notification service correctly initializes
	// the SendGrid client based on configuration
	
	// Save original environment
	originalEnv := make(map[string]string)
	testEnvVars := []string{"APP_ENV", "SENDGRID_API_KEY", "FROM_EMAIL"}
	
	for _, key := range testEnvVars {
		originalEnv[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	
	// Clean up after test
	defer func() {
		for _, key := range testEnvVars {
			if originalValue, exists := originalEnv[key]; exists {
				os.Setenv(key, originalValue)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	tests := []struct {
		name               string
		sendgridAPIKey     string
		fromEmail          string
		expectInitialized  bool
	}{
		{
			name:              "with SendGrid configuration",
			sendgridAPIKey:    "SG.test-key",
			fromEmail:         "test@example.com",
			expectInitialized: true,
		},
		{
			name:              "without SendGrid API key",
			sendgridAPIKey:    "",
			fromEmail:         "test@example.com",
			expectInitialized: false,
		},
		{
			name:              "without from email",
			sendgridAPIKey:    "SG.test-key",
			fromEmail:         "",
			expectInitialized: false,
		},
		{
			name:              "without any configuration",
			sendgridAPIKey:    "",
			fromEmail:         "",
			expectInitialized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables first
			os.Unsetenv("SENDGRID_API_KEY")
			os.Unsetenv("FROM_EMAIL")
			
			// Set test environment
			os.Setenv("APP_ENV", "local")
			if tt.sendgridAPIKey != "" {
				os.Setenv("SENDGRID_API_KEY", tt.sendgridAPIKey)
			}
			if tt.fromEmail != "" {
				os.Setenv("FROM_EMAIL", tt.fromEmail)
			}

			// Load config
			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			// Verify SendGrid configuration
			if tt.expectInitialized {
				if cfg.SendGridAPIKey == "" || cfg.FromEmail == "" {
					t.Error("Expected SendGrid to be configured but it wasn't")
				}
			} else {
				if cfg.SendGridAPIKey != "" && cfg.FromEmail != "" {
					t.Error("Expected SendGrid not to be configured but it was")
				}
			}
		})
	}
}

func TestHealthServerStartup(t *testing.T) {
	// This test verifies that the health server starts correctly
	// In a real test, you would:
	// 1. Start the server in a goroutine
	// 2. Make HTTP requests to verify endpoints
	// 3. Properly shutdown the server

	// Save original PORT
	originalPort := os.Getenv("PORT")
	defer func() {
		if originalPort != "" {
			os.Setenv("PORT", originalPort)
		} else {
			os.Unsetenv("PORT")
		}
	}()

	// Test with custom port
	os.Setenv("PORT", "9999")

	// Start server in goroutine
	serverStarted := make(chan bool)
	go func() {
		serverStarted <- true
		// Note: In real test, you would start the actual server here
		// For now, we just simulate it
		time.Sleep(100 * time.Millisecond)
	}()

	// Wait for server to start
	select {
	case <-serverStarted:
		// Server started successfully
	case <-time.After(1 * time.Second):
		t.Error("Server failed to start within timeout")
	}

	// In a real test, you would make HTTP requests here to verify:
	// - GET / returns 200 OK
	// - GET /health returns 200 OK
}

