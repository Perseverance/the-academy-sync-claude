package services

import (
	"context"
	"testing"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

func TestConfigService_extractSpreadsheetID(t *testing.T) {
	// Create a test logger
	testLogger := logger.New("config_service_test")
	service := &ConfigService{
		logger: testLogger,
	}

	tests := []struct {
		name        string
		url         string
		expectedID  string
		expectError bool
	}{
		{
			name:        "Valid full URL with edit and gid",
			url:         "https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit#gid=0",
			expectedID:  "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
			expectError: false,
		},
		{
			name:        "Valid URL with edit only",
			url:         "https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit",
			expectedID:  "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
			expectError: false,
		},
		{
			name:        "Valid URL without edit",
			url:         "https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
			expectedID:  "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
			expectError: false,
		},
		{
			name:        "URL without https protocol",
			url:         "docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit",
			expectedID:  "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
			expectError: false,
		},
		{
			name:        "URL with whitespace",
			url:         "  https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit  ",
			expectedID:  "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
			expectError: false,
		},
		{
			name:        "Empty URL",
			url:         "",
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "Whitespace only URL",
			url:         "   ",
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "Invalid URL - not Google Sheets",
			url:         "https://www.example.com/spreadsheet",
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "Invalid URL - Google Docs instead of Sheets",
			url:         "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit",
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "Malformed Google Sheets URL",
			url:         "https://docs.google.com/spreadsheets/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "URL with special characters in ID",
			url:         "https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms-_test/edit",
			expectedID:  "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms-_test",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.extractSpreadsheetID(tt.url)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none for URL: %s", tt.url)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for URL %s: %v", tt.url, err)
				return
			}

			if result != tt.expectedID {
				t.Errorf("Expected ID %s but got %s for URL: %s", tt.expectedID, result, tt.url)
			}
		})
	}
}

func TestConfigService_sanitizeURL(t *testing.T) {
	// Create a test logger  
	testLogger := logger.New("config_service_test")
	service := &ConfigService{
		logger: testLogger,
	}

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Google Sheets URL",
			url:      "https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit",
			expected: "https://docs.google.com/spreadsheets/d/[ID]...",
		},
		{
			name:     "Other Google URL",
			url:      "https://docs.google.com/document/d/123/edit",
			expected: "https://docs.google.com/...",
		},
		{
			name:     "Long non-Google URL",
			url:      "https://www.example.com/very/long/path/that/should/be/truncated/for/privacy/reasons",
			expected: "https://www.example.com/very/l...",
		},
		{
			name:     "Short URL",
			url:      "https://example.com",
			expected: "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.sanitizeURL(tt.url)
			if result != tt.expected {
				t.Errorf("Expected %s but got %s", tt.expected, result)
			}
		})
	}
}

func TestConfigError(t *testing.T) {
	tests := []struct {
		name          string
		errorType     string
		message       string
		cause         error
		expectedError string
	}{
		{
			name:          "Error without cause",
			errorType:     ConfigErrorInvalidURL,
			message:       "Invalid URL format",
			cause:         nil,
			expectedError: "INVALID_URL: Invalid URL format",
		},
		{
			name:          "Error with cause",
			errorType:     ConfigErrorPermission,
			message:       "Permission denied",
			cause:         context.DeadlineExceeded,
			expectedError: "PERMISSION_ERROR: Permission denied (caused by: context deadline exceeded)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ConfigError{
				Type:    tt.errorType,
				Message: tt.message,
				Cause:   tt.cause,
			}

			if err.Error() != tt.expectedError {
				t.Errorf("Expected error string %s but got %s", tt.expectedError, err.Error())
			}
		})
	}
}