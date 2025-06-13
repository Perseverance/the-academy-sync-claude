// Package config provides a hybrid configuration loading mechanism that supports
// both local development (.env files) and production environments (Google Secret Manager).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config represents the application configuration with all required settings.
type Config struct {
	// Environment settings
	Environment string `json:"environment"`
	Port        string `json:"port"`

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

	// SMTP configuration
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     string `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	FromEmail    string `json:"from_email"`

	// GCP configuration
	GCPProjectID string `json:"gcp_project_id"`
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

		// SMTP
		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		FromEmail:    getEnv("FROM_EMAIL", ""),

		// GCP
		GCPProjectID: getEnv("GCP_PROJECT_ID", ""),
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

	// Validate required configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// loadFromSecretManager loads configuration from Google Secret Manager.
// For now, this is a placeholder that falls back to environment variables.
// Full Secret Manager integration will be added when deploying to GCP.
func loadFromSecretManager() (*Config, error) {
	// For now, fall back to environment variable loading
	// This ensures the code works in production environments that use env vars
	// Full Google Secret Manager integration can be added later
	
	config := &Config{
		Environment: getEnv("APP_ENV", "production"),
		Port:        getEnv("PORT", "8080"),

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

		// SMTP
		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		FromEmail:    getEnv("FROM_EMAIL", ""),

		// GCP
		GCPProjectID: getEnv("GCP_PROJECT_ID", ""),
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

	// Validate port is numeric
	if c.Port != "" {
		if _, err := strconv.Atoi(c.Port); err != nil {
			errors = append(errors, "port must be a valid number")
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

