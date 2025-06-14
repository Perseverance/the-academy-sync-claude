package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	testEnvVars := []string{
		"APP_ENV", "PORT", "POSTGRES_DB", "POSTGRES_USER", "POSTGRES_PASSWORD",
		"POSTGRES_HOST", "POSTGRES_PORT", "REDIS_HOST", "REDIS_PORT",
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "JWT_SECRET",
	}

	for _, key := range testEnvVars {
		originalEnv[key] = os.Getenv(key)
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

	// Set test environment variables
	testValues := map[string]string{
		"APP_ENV":             "local",
		"PORT":                "9090",
		"POSTGRES_DB":         "test_db",
		"POSTGRES_USER":       "test_user",
		"POSTGRES_PASSWORD":   "test_pass",
		"POSTGRES_HOST":       "test_host",
		"POSTGRES_PORT":       "5432",
		"REDIS_HOST":          "redis_host",
		"REDIS_PORT":          "6379",
		"GOOGLE_CLIENT_ID":    "test_google_id",
		"GOOGLE_CLIENT_SECRET": "test_google_secret",
		"JWT_SECRET":          "test_jwt_secret",
	}

	for key, value := range testValues {
		os.Setenv(key, value)
	}

	// Test loading configuration
	config, err := loadFromEnv()
	if err != nil {
		t.Fatalf("loadFromEnv() failed: %v", err)
	}

	// Verify configuration values
	if config.Environment != "local" {
		t.Errorf("Expected Environment to be 'local', got '%s'", config.Environment)
	}

	if config.Port != "9090" {
		t.Errorf("Expected Port to be '9090', got '%s'", config.Port)
	}

	if config.PostgresDB != "test_db" {
		t.Errorf("Expected PostgresDB to be 'test_db', got '%s'", config.PostgresDB)
	}

	if config.GoogleClientID != "test_google_id" {
		t.Errorf("Expected GoogleClientID to be 'test_google_id', got '%s'", config.GoogleClientID)
	}

	// Verify database URL construction
	expectedDBURL := "postgres://test_user:test_pass@test_host:5432/test_db?sslmode=disable"
	if config.DatabaseURL != expectedDBURL {
		t.Errorf("Expected DatabaseURL to be '%s', got '%s'", expectedDBURL, config.DatabaseURL)
	}

	// Verify Redis URL construction
	expectedRedisURL := "redis://redis_host:6379"
	if config.RedisURL != expectedRedisURL {
		t.Errorf("Expected RedisURL to be '%s', got '%s'", expectedRedisURL, config.RedisURL)
	}
}

func TestLoadFromEnvWithDefaults(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	testEnvVars := []string{"APP_ENV", "PORT", "POSTGRES_DB", "POSTGRES_HOST"}

	for _, key := range testEnvVars {
		originalEnv[key] = os.Getenv(key)
		os.Unsetenv(key) // Clear all env vars to test defaults
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

	config, err := loadFromEnv()
	if err != nil {
		t.Fatalf("loadFromEnv() failed: %v", err)
	}

	// Verify default values
	if config.Environment != "local" {
		t.Errorf("Expected default Environment to be 'local', got '%s'", config.Environment)
	}

	if config.Port != "8080" {
		t.Errorf("Expected default Port to be '8080', got '%s'", config.Port)
	}

	if config.PostgresDB != "academy_sync" {
		t.Errorf("Expected default PostgresDB to be 'academy_sync', got '%s'", config.PostgresDB)
	}

	if config.PostgresHost != "localhost" {
		t.Errorf("Expected default PostgresHost to be 'localhost', got '%s'", config.PostgresHost)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid local config",
			config: Config{
				Environment: "local",
				Port:        "8080",
			},
			expectError: false,
		},
		{
			name: "valid production config",
			config: Config{
				Environment: "production",
				Port:        "8080",
				JWTSecret:   "secret",
				EncryptionSecret: "this-is-a-32-character-encryption-secret-key",
			},
			expectError: false,
		},
		{
			name: "missing environment",
			config: Config{
				Port: "8080",
			},
			expectError: true,
			errorMsg:    "environment is required",
		},
		{
			name: "missing port",
			config: Config{
				Environment: "local",
			},
			expectError: true,
			errorMsg:    "port is required",
		},
		{
			name: "production without JWT secret",
			config: Config{
				Environment: "production",
				Port:        "8080",
			},
			expectError: true,
			errorMsg:    "JWT_SECRET is required in production",
		},
		{
			name: "production without encryption secret",
			config: Config{
				Environment: "production",
				Port:        "8080",
				JWTSecret:   "secret",
			},
			expectError: true,
			errorMsg:    "ENCRYPTION_SECRET is required in production",
		},
		{
			name: "encryption secret too short",
			config: Config{
				Environment: "local",
				Port:        "8080",
				EncryptionSecret: "short",
			},
			expectError: true,
			errorMsg:    "ENCRYPTION_SECRET should be at least 32 characters",
		},
		{
			name: "invalid port",
			config: Config{
				Environment: "local",
				Port:        "invalid",
			},
			expectError: true,
			errorMsg:    "port must be a valid number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			
			if tt.expectError && err == nil {
				t.Errorf("Expected validation error but got none")
			}
			
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
			
			if tt.expectError && err != nil && tt.errorMsg != "" {
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Save original environment
	originalAppEnv := os.Getenv("APP_ENV")
	originalGoEnv := os.Getenv("GO_ENV")

	// Clean up after test
	defer func() {
		if originalAppEnv != "" {
			os.Setenv("APP_ENV", originalAppEnv)
		} else {
			os.Unsetenv("APP_ENV")
		}
		if originalGoEnv != "" {
			os.Setenv("GO_ENV", originalGoEnv)
		} else {
			os.Unsetenv("GO_ENV")
		}
	}()

	tests := []struct {
		name        string
		appEnv      string
		goEnv       string
		expectLocal bool
	}{
		{
			name:        "local environment",
			appEnv:      "local",
			expectLocal: true,
		},
		{
			name:        "development environment",
			appEnv:      "development",
			expectLocal: true,
		},
		{
			name:        "dev environment",
			appEnv:      "dev",
			expectLocal: true,
		},
		{
			name:        "fallback to GO_ENV local",
			goEnv:       "local",
			expectLocal: true,
		},
		{
			name:        "default to local",
			expectLocal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear both env vars first
			os.Unsetenv("APP_ENV")
			os.Unsetenv("GO_ENV")

			// Set test values
			if tt.appEnv != "" {
				os.Setenv("APP_ENV", tt.appEnv)
			}
			if tt.goEnv != "" {
				os.Setenv("GO_ENV", tt.goEnv)
			}

			config, err := Load()
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}

			if tt.expectLocal {
				if config.Environment != "local" && config.Environment != "development" && config.Environment != "dev" {
					t.Errorf("Expected local environment, got '%s'", config.Environment)
				}
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	// Save original value
	originalValue := os.Getenv("TEST_VAR")
	defer func() {
		if originalValue != "" {
			os.Setenv("TEST_VAR", originalValue)
		} else {
			os.Unsetenv("TEST_VAR")
		}
	}()

	// Test with environment variable set
	os.Setenv("TEST_VAR", "test_value")
	result := getEnv("TEST_VAR", "default")
	if result != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", result)
	}

	// Test with environment variable unset
	os.Unsetenv("TEST_VAR")
	result = getEnv("TEST_VAR", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got '%s'", result)
	}
}

func TestGetValueOrEnv(t *testing.T) {
	// Save original value
	originalValue := os.Getenv("TEST_VAR")
	defer func() {
		if originalValue != "" {
			os.Setenv("TEST_VAR", originalValue)
		} else {
			os.Unsetenv("TEST_VAR")
		}
	}()

	os.Setenv("TEST_VAR", "env_value")

	// Test with secret value available
	secretValue := "secret_value"
	result := getValueOrEnv(&secretValue, "TEST_VAR", "default")
	if result != "secret_value" {
		t.Errorf("Expected 'secret_value', got '%s'", result)
	}

	// Test with empty secret value, should fall back to env
	emptySecret := ""
	result = getValueOrEnv(&emptySecret, "TEST_VAR", "default")
	if result != "env_value" {
		t.Errorf("Expected 'env_value', got '%s'", result)
	}

	// Test with nil secret value, should fall back to env
	result = getValueOrEnv(nil, "TEST_VAR", "default")
	if result != "env_value" {
		t.Errorf("Expected 'env_value', got '%s'", result)
	}

	// Test with no secret and no env, should use default
	os.Unsetenv("TEST_VAR")
	result = getValueOrEnv(nil, "TEST_VAR", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got '%s'", result)
	}
}

func TestLoadFromEnvForProduction(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	testEnvVars := []string{
		"APP_ENV", "PORT", "DATABASE_URL", "JWT_SECRET", "ENCRYPTION_SECRET",
	}

	for _, key := range testEnvVars {
		originalEnv[key] = os.Getenv(key)
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

	// Set test environment variables
	testValues := map[string]string{
		"APP_ENV":           "production",
		"PORT":              "9090",
		"DATABASE_URL":      "postgres://test:test@localhost:5432/test",
		"JWT_SECRET":        "production-secret",
		"ENCRYPTION_SECRET": "this-is-a-32-character-encryption-secret-key",
	}

	for key, value := range testValues {
		os.Setenv(key, value)
	}

	// Test loading configuration
	config, err := loadFromEnvForProduction()
	if err != nil {
		t.Fatalf("loadFromEnvForProduction() failed: %v", err)
	}

	// Verify configuration values
	if config.Environment != "production" {
		t.Errorf("Expected Environment to be 'production', got '%s'", config.Environment)
	}

	if config.DatabaseURL != "postgres://test:test@localhost:5432/test" {
		t.Errorf("Expected DatabaseURL to be set correctly, got '%s'", config.DatabaseURL)
	}

	if config.JWTSecret != "production-secret" {
		t.Errorf("Expected JWTSecret to be 'production-secret', got '%s'", config.JWTSecret)
	}
}

func TestLoadFromSecretManagerFallback(t *testing.T) {
	// Save original environment
	originalGCPProject := os.Getenv("GCP_PROJECT_ID")
	originalAppEnv := os.Getenv("APP_ENV")
	originalJWT := os.Getenv("JWT_SECRET")
	originalEncryption := os.Getenv("ENCRYPTION_SECRET")

	// Clean up after test
	defer func() {
		if originalGCPProject != "" {
			os.Setenv("GCP_PROJECT_ID", originalGCPProject)
		} else {
			os.Unsetenv("GCP_PROJECT_ID")
		}
		if originalAppEnv != "" {
			os.Setenv("APP_ENV", originalAppEnv)
		} else {
			os.Unsetenv("APP_ENV")
		}
		if originalJWT != "" {
			os.Setenv("JWT_SECRET", originalJWT)
		} else {
			os.Unsetenv("JWT_SECRET")
		}
		if originalEncryption != "" {
			os.Setenv("ENCRYPTION_SECRET", originalEncryption)
		} else {
			os.Unsetenv("ENCRYPTION_SECRET")
		}
	}()

	// Test case 1: Missing GCP_PROJECT_ID should return error
	os.Unsetenv("GCP_PROJECT_ID")
	_, err := loadFromSecretManager()
	if err == nil {
		t.Error("Expected error when GCP_PROJECT_ID is missing")
	}
	if !strings.Contains(err.Error(), "GCP_PROJECT_ID environment variable is required") {
		t.Errorf("Expected specific error message, got: %v", err)
	}

	// Test case 2: With GCP_PROJECT_ID set, should attempt Secret Manager
	// (but will fall back to env vars since we don't have real GCP credentials)
	os.Setenv("GCP_PROJECT_ID", "test-project")
	os.Setenv("APP_ENV", "production")
	os.Setenv("JWT_SECRET", "fallback-secret")
	os.Setenv("ENCRYPTION_SECRET", "this-is-a-32-character-encryption-secret-key")

	config, err := loadFromSecretManager()
	if err != nil {
		t.Fatalf("loadFromSecretManager() should not fail with fallback: %v", err)
	}

	if config.GCPProjectID != "test-project" {
		t.Errorf("Expected GCPProjectID to be 'test-project', got '%s'", config.GCPProjectID)
	}

	if config.JWTSecret != "fallback-secret" {
		t.Errorf("Expected JWTSecret fallback to work, got '%s'", config.JWTSecret)
	}
}

