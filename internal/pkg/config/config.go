// Package config provides a hybrid configuration loading mechanism that supports
// both local development (.env files) and production environments (Google Secret Manager).
package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/joho/godotenv"
)

// Config represents the application configuration with all required settings.
type Config struct {
	// Environment settings
	Environment string `json:"environment"`
	Port        string `json:"port"`
	BaseURL     string `json:"base_url"`
	FrontendURL string `json:"frontend_url"`
	LogLevel    string `json:"log_level"`

	// Database configuration
	DatabaseURL      string `json:"database_url"`
	PostgresDB       string `json:"postgres_db"`
	PostgresUser     string `json:"postgres_user"`
	PostgresPassword string `json:"postgres_password"`
	PostgresHost     string `json:"postgres_host"`
	PostgresPort     string `json:"postgres_port"`

	// Redis configuration
	RedisURL  string `json:"redis_url"`
	RedisHost string `json:"redis_host"`
	RedisPort string `json:"redis_port"`

	// OAuth credentials
	GoogleClientID     string `json:"google_client_id"`
	GoogleClientSecret string `json:"google_client_secret"`
	StravaClientID     string `json:"strava_client_id"`
	StravaClientSecret string `json:"strava_client_secret"`

	// JWT configuration
	JWTSecret string `json:"jwt_secret"`

	// Encryption configuration
	EncryptionSecret string `json:"encryption_secret"`

	// SMTP configuration
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     string `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	FromEmail    string `json:"from_email"`

	// GCP configuration
	GCPProjectID string `json:"gcp_project_id"`

	// Fail-fast configuration
	FailFastEnabled bool `json:"fail_fast_enabled"`
}

// Load loads configuration based on the environment.
// In local environments (APP_ENV=local), it loads from .env file.
// In production environments, it loads from Google Secret Manager.
func Load() (*Config, error) {
	env := getEnv("APP_ENV", getEnv("GO_ENV", "local"))

	switch env {
	case "local", "development", "dev":
		return loadFromEnv()
	case "production", "prod", "staging":
		return loadFromSecretManager()
	default:
		return loadFromEnv() // Default to local for unknown environments
	}
}

// loadFromEnv loads configuration from environment variables and .env file.
func loadFromEnv() (*Config, error) {
	// Try to load .env file if it exists (ignore errors as it's optional)
	_ = godotenv.Load()

	config := &Config{
		Environment: getEnv("APP_ENV", getEnv("GO_ENV", "local")),
		Port:        getEnv("PORT", "8080"),
		BaseURL:     getEnv("BASE_URL", ""),
		FrontendURL: getEnv("FRONTEND_URL", ""),
		LogLevel:    getEnv("LOG_LEVEL", "INFO"),

		// Database
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		PostgresDB:       getEnv("POSTGRES_DB", "academy_sync"),
		PostgresUser:     getEnv("POSTGRES_USER", "postgres"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "postgres"),
		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnv("POSTGRES_PORT", "5433"),

		// Redis
		RedisURL:  getEnv("REDIS_URL", ""),
		RedisHost: getEnv("REDIS_HOST", "localhost"),
		RedisPort: getEnv("REDIS_PORT", "6380"),

		// OAuth
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		StravaClientID:     getEnv("STRAVA_CLIENT_ID", ""),
		StravaClientSecret: getEnv("STRAVA_CLIENT_SECRET", ""),

		// JWT
		JWTSecret: getEnv("JWT_SECRET", ""),

		// Encryption
		EncryptionSecret: getEnv("ENCRYPTION_SECRET", ""),

		// SMTP
		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		FromEmail:    getEnv("FROM_EMAIL", ""),

		// GCP
		GCPProjectID: getEnv("GCP_PROJECT_ID", ""),

		// Fail-fast
		FailFastEnabled: getEnvBool("FAIL_FAST_ENABLED", false),
	}

	// Build database URL if not provided
	if config.DatabaseURL == "" && config.PostgresHost != "" {
		config.DatabaseURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=disable",
			config.PostgresUser,
			config.PostgresPassword,
			config.PostgresHost,
			config.PostgresPort,
			config.PostgresDB,
		)
	}

	// Build Redis URL if not provided
	if config.RedisURL == "" && config.RedisHost != "" {
		config.RedisURL = fmt.Sprintf("redis://%s:%s", config.RedisHost, config.RedisPort)
	}

	// Build BaseURL if not provided
	config.buildBaseURL()

	// Build FrontendURL if not provided
	config.buildFrontendURL()

	// Validate required configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// loadFromSecretManager loads configuration from Google Secret Manager.
func loadFromSecretManager() (*Config, error) {
	ctx := context.Background()
	
	projectID := getEnv("GCP_PROJECT_ID", "")
	if projectID == "" {
		return nil, fmt.Errorf("GCP_PROJECT_ID environment variable is required for Secret Manager")
	}

	// Try to create Secret Manager client
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		// If Secret Manager is not available, fall back to environment variables
		// This allows graceful degradation in environments without Secret Manager access
		fmt.Printf("Warning: Could not create Secret Manager client (%v), falling back to environment variables\n", err)
		return loadFromEnvForProduction()
	}
	defer client.Close()

	fmt.Printf("Info: Loading configuration from Google Secret Manager for project: %s\n", projectID)

	// Define secrets to fetch from Secret Manager
	secrets := map[string]*string{
		"database-url":           new(string),
		"redis-url":              new(string),
		"google-client-id":       new(string),
		"google-client-secret":   new(string),
		"strava-client-id":       new(string),
		"strava-client-secret":   new(string),
		"jwt-secret":             new(string),
		"encryption-secret":      new(string),
		"smtp-username":          new(string),
		"smtp-password":          new(string),
		"from-email":             new(string),
		"database-password":      new(string),
	}

	// Fetch each secret
	secretsLoaded := 0
	for secretName, value := range secrets {
		secretValue, err := getSecret(ctx, client, projectID, secretName)
		if err != nil {
			// Log warning but don't fail for optional secrets
			fmt.Printf("Warning: failed to get secret %s: %v\n", secretName, err)
			continue
		}
		*value = secretValue
		secretsLoaded++
	}

	fmt.Printf("Info: Successfully loaded %d secrets from Secret Manager\n", secretsLoaded)

	config := &Config{
		Environment: getEnv("APP_ENV", "production"),
		Port:        getEnv("PORT", "8080"),
		BaseURL:     getEnv("BASE_URL", ""),
		FrontendURL: getEnv("FRONTEND_URL", ""),
		LogLevel:    getEnv("LOG_LEVEL", "INFO"),

		// Use secrets if available, otherwise fall back to env vars
		DatabaseURL:        getValueOrEnv(secrets["database-url"], "DATABASE_URL", ""),
		RedisURL:           getValueOrEnv(secrets["redis-url"], "REDIS_URL", ""),
		GoogleClientID:     getValueOrEnv(secrets["google-client-id"], "GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getValueOrEnv(secrets["google-client-secret"], "GOOGLE_CLIENT_SECRET", ""),
		StravaClientID:     getValueOrEnv(secrets["strava-client-id"], "STRAVA_CLIENT_ID", ""),
		StravaClientSecret: getValueOrEnv(secrets["strava-client-secret"], "STRAVA_CLIENT_SECRET", ""),
		JWTSecret:          getValueOrEnv(secrets["jwt-secret"], "JWT_SECRET", ""),
		EncryptionSecret:   getValueOrEnv(secrets["encryption-secret"], "ENCRYPTION_SECRET", ""),
		SMTPUsername:       getValueOrEnv(secrets["smtp-username"], "SMTP_USERNAME", ""),
		SMTPPassword:       getValueOrEnv(secrets["smtp-password"], "SMTP_PASSWORD", ""),
		FromEmail:          getValueOrEnv(secrets["from-email"], "FROM_EMAIL", ""),

		// These typically come from environment in GCP
		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		GCPProjectID: projectID,

		// Database components (for URL construction if needed)
		PostgresDB:   getEnv("POSTGRES_DB", "academy_sync"),
		PostgresUser: getEnv("POSTGRES_USER", "postgres"),
		PostgresHost: getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort: getEnv("POSTGRES_PORT", "5432"),

		// Redis components (for URL construction if needed)
		RedisHost: getEnv("REDIS_HOST", "localhost"),
		RedisPort: getEnv("REDIS_PORT", "6379"),

		// Fail-fast
		FailFastEnabled: getEnvBool("FAIL_FAST_ENABLED", false),
	}

	// Build database URL if not provided from secrets
	if config.DatabaseURL == "" {
		// Try to get database password from secrets or env
		dbPassword := getValueOrEnv(secrets["database-password"], "POSTGRES_PASSWORD", "")
		if dbPassword != "" {
			config.PostgresPassword = dbPassword
			config.DatabaseURL = fmt.Sprintf(
				"postgres://%s:%s@%s:%s/%s?sslmode=disable",
				config.PostgresUser,
				config.PostgresPassword,
				config.PostgresHost,
				config.PostgresPort,
				config.PostgresDB,
			)
		}
	}

	// Build Redis URL if not provided from secrets
	if config.RedisURL == "" && config.RedisHost != "" {
		config.RedisURL = fmt.Sprintf("redis://%s:%s", config.RedisHost, config.RedisPort)
	}

	// Build BaseURL if not provided
	config.buildBaseURL()

	// Build FrontendURL if not provided
	config.buildFrontendURL()

	// Validate required configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// getSecret retrieves a secret from Google Secret Manager.
func getSecret(ctx context.Context, client *secretmanager.Client, projectID, secretName string) (string, error) {
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectID, secretName),
	}

	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %s: %w", secretName, err)
	}

	return string(result.Payload.Data), nil
}

// loadFromEnvForProduction loads configuration from environment variables for production.
// This is used as a fallback when Secret Manager is not available or when running
// in environments that use environment variables instead of Secret Manager.
func loadFromEnvForProduction() (*Config, error) {
	config := &Config{
		Environment: getEnv("APP_ENV", "production"),
		Port:        getEnv("PORT", "8080"),
		BaseURL:     getEnv("BASE_URL", ""),
		FrontendURL: getEnv("FRONTEND_URL", ""),
		LogLevel:    getEnv("LOG_LEVEL", "INFO"),

		// Database
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		PostgresDB:       getEnv("POSTGRES_DB", "academy_sync"),
		PostgresUser:     getEnv("POSTGRES_USER", "postgres"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", ""),
		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnv("POSTGRES_PORT", "5432"),

		// Redis
		RedisURL:  getEnv("REDIS_URL", ""),
		RedisHost: getEnv("REDIS_HOST", "localhost"),
		RedisPort: getEnv("REDIS_PORT", "6379"),

		// OAuth
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		StravaClientID:     getEnv("STRAVA_CLIENT_ID", ""),
		StravaClientSecret: getEnv("STRAVA_CLIENT_SECRET", ""),

		// JWT
		JWTSecret: getEnv("JWT_SECRET", ""),

		// Encryption
		EncryptionSecret: getEnv("ENCRYPTION_SECRET", ""),

		// SMTP
		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		FromEmail:    getEnv("FROM_EMAIL", ""),

		// GCP
		GCPProjectID: getEnv("GCP_PROJECT_ID", ""),

		// Fail-fast
		FailFastEnabled: getEnvBool("FAIL_FAST_ENABLED", false),
	}

	// Build database URL if not provided
	if config.DatabaseURL == "" && config.PostgresHost != "" {
		config.DatabaseURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=disable",
			config.PostgresUser,
			config.PostgresPassword,
			config.PostgresHost,
			config.PostgresPort,
			config.PostgresDB,
		)
	}

	// Build Redis URL if not provided
	if config.RedisURL == "" && config.RedisHost != "" {
		config.RedisURL = fmt.Sprintf("redis://%s:%s", config.RedisHost, config.RedisPort)
	}

	// Build BaseURL if not provided
	config.buildBaseURL()

	// Build FrontendURL if not provided
	config.buildFrontendURL()

	// Validate required configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// validate performs basic validation on the configuration.
func (c *Config) validate() error {
	var errors []string

	// Check critical configuration
	if c.Environment == "" {
		errors = append(errors, "environment is required")
	}

	if c.Port == "" {
		errors = append(errors, "port is required")
	}

	// In production, JWT secret is critical
	if c.Environment != "local" && c.Environment != "development" && c.Environment != "dev" && c.JWTSecret == "" {
		errors = append(errors, "JWT_SECRET is required in production")
	}

	// In production, encryption secret is critical
	if c.Environment != "local" && c.Environment != "development" && c.Environment != "dev" && c.EncryptionSecret == "" {
		errors = append(errors, "ENCRYPTION_SECRET is required in production")
	}

	// In production, BaseURL must be explicitly configured
	if c.Environment != "local" && c.Environment != "development" && c.Environment != "dev" && c.BaseURL == "" {
		errors = append(errors, "BASE_URL is required in production environments")
	}

	// Validate encryption secret length (recommended minimum 32 bytes)
	if c.EncryptionSecret != "" && len(c.EncryptionSecret) < 32 {
		errors = append(errors, "ENCRYPTION_SECRET should be at least 32 characters for security")
	}

	// Validate port is numeric
	if c.Port != "" {
		if _, err := strconv.Atoi(c.Port); err != nil {
			errors = append(errors, "port must be a valid number")
		}
	}

	// Validate log level
	if c.LogLevel != "" {
		validLevels := map[string]struct{}{
			"DEBUG": {}, "INFO": {}, "WARN": {}, "WARNING": {},
			"ERROR": {}, "CRITICAL": {},
		}
		if _, ok := validLevels[strings.ToUpper(c.LogLevel)]; !ok {
			errors = append(errors, "log_level must be one of DEBUG, INFO, WARN, WARNING, ERROR, CRITICAL")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, ", "))
	}

	return nil
}

// getEnv gets an environment variable with a fallback default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool gets a boolean environment variable with a fallback default value.
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch strings.ToLower(value) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return defaultValue
}

// getValueOrEnv returns the secret value if available, otherwise falls back to environment variable.
func getValueOrEnv(secretValue *string, envKey, defaultValue string) string {
	if secretValue != nil && *secretValue != "" {
		return *secretValue
	}
	return getEnv(envKey, defaultValue)
}

// buildBaseURL constructs the base URL if not provided for development environments only
func (c *Config) buildBaseURL() {
	if c.BaseURL == "" {
		if c.Environment == "local" || c.Environment == "development" || c.Environment == "dev" {
			c.BaseURL = fmt.Sprintf("http://localhost:%s", c.Port)
		}
		// In production, BaseURL must be explicitly configured - no fallback provided
	}
}

// buildFrontendURL constructs the frontend URL if not provided for development environments only
func (c *Config) buildFrontendURL() {
	if c.FrontendURL == "" {
		if c.Environment == "local" || c.Environment == "development" || c.Environment == "dev" {
			c.FrontendURL = "http://localhost:3000"
		}
		// In production, FrontendURL must be explicitly configured - no fallback provided
	}
}

