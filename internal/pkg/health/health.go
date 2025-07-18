package health

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// HealthChecker provides functionality to check the health of various dependencies
type HealthChecker struct {
	log            *logger.Logger
	sendGridAPIURL string // Allow overriding for testing
}

// HealthCheckResult represents the result of a health check operation
type HealthCheckResult struct {
	Service string
	Status  string
	Error   error
	Latency time.Duration
}

// NewHealthChecker creates a new health checker instance
func NewHealthChecker(log *logger.Logger) *HealthChecker {
	return &HealthChecker{
		log:            log,
		sendGridAPIURL: "https://api.sendgrid.com",
	}
}

// CheckDatabase performs a health check on the database connection
// Returns a result indicating whether the database is healthy and responsive
func (h *HealthChecker) CheckDatabase(ctx context.Context, db *sql.DB) *HealthCheckResult {
	start := time.Now()
	result := &HealthCheckResult{
		Service: "database",
		Status:  "healthy",
	}

	// Create timeout context for the health check
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Attempt to ping the database
	if err := db.PingContext(checkCtx); err != nil {
		result.Status = "unhealthy"
		result.Error = err
		h.log.Error("Database health check failed",
			"error", err.Error(),
			"latency_ms", time.Since(start).Milliseconds())
	} else {
		h.log.Debug("Database health check passed",
			"latency_ms", time.Since(start).Milliseconds())
	}

	result.Latency = time.Since(start)
	return result
}

// CheckDatabaseConnection validates database connectivity with connection establishment
// This is more comprehensive than CheckDatabase as it also validates connection creation
func (h *HealthChecker) CheckDatabaseConnection(ctx context.Context, databaseURL string) *HealthCheckResult {
	start := time.Now()
	result := &HealthCheckResult{
		Service: "database_connection",
		Status:  "healthy",
	}

	// Create timeout context for the health check
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Open database connection
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		result.Status = "unhealthy"
		result.Error = fmt.Errorf("failed to open database connection: %w", err)
		result.Latency = time.Since(start)
		h.log.Error("Database connection establishment failed",
			"error", err.Error(),
			"latency_ms", result.Latency.Milliseconds())
		return result
	}
	defer db.Close()

	// Test the connection with ping
	if err := db.PingContext(checkCtx); err != nil {
		result.Status = "unhealthy"
		result.Error = fmt.Errorf("database ping failed: %w", err)
		h.log.Error("Database ping failed during health check",
			"error", err.Error(),
			"latency_ms", time.Since(start).Milliseconds())
	} else {
		h.log.Debug("Database connection health check passed",
			"latency_ms", time.Since(start).Milliseconds())
	}

	result.Latency = time.Since(start)
	return result
}

// IsHealthy returns true if the health check result indicates a healthy service
func (r *HealthCheckResult) IsHealthy() bool {
	return r.Status == "healthy" && r.Error == nil
}

// String returns a formatted string representation of the health check result
func (r *HealthCheckResult) String() string {
	if r.IsHealthy() {
		return fmt.Sprintf("Service '%s' is healthy (latency: %v)", r.Service, r.Latency)
	}
	return fmt.Sprintf("Service '%s' is unhealthy: %v (latency: %v)", r.Service, r.Error, r.Latency)
}

// CheckSendGrid performs a health check on the SendGrid API connectivity
// It validates the API key by making a simple authenticated request to the SendGrid API
func (h *HealthChecker) CheckSendGrid(ctx context.Context, apiKey string) *HealthCheckResult {
	start := time.Now()
	result := &HealthCheckResult{
		Service: "sendgrid",
		Status:  "healthy",
	}

	// Create timeout context for the health check
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Create HTTP client
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create request to SendGrid API - using the authenticated user endpoint
	// This is a simple GET request that requires authentication but doesn't send any emails
	req, err := http.NewRequestWithContext(checkCtx, "GET", h.sendGridAPIURL+"/v3/user/profile", nil)
	if err != nil {
		result.Status = "unhealthy"
		result.Error = fmt.Errorf("failed to create request: %w", err)
		result.Latency = time.Since(start)
		h.log.Error("SendGrid health check failed - request creation",
			"error", err.Error(),
			"latency_ms", result.Latency.Milliseconds())
		return result
	}

	// Set authorization header
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		result.Status = "unhealthy"
		result.Error = fmt.Errorf("failed to connect to SendGrid: %w", err)
		result.Latency = time.Since(start)
		h.log.Error("SendGrid health check failed - connection error",
			"error", err.Error(),
			"latency_ms", result.Latency.Milliseconds())
		return result
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		result.Status = "unhealthy"
		result.Error = fmt.Errorf("invalid SendGrid API key")
		h.log.Error("SendGrid health check failed - authentication error",
			"status_code", resp.StatusCode,
			"latency_ms", time.Since(start).Milliseconds())
	} else if resp.StatusCode >= 400 {
		result.Status = "unhealthy"
		result.Error = fmt.Errorf("SendGrid API returned error status: %d", resp.StatusCode)
		h.log.Error("SendGrid health check failed - API error",
			"status_code", resp.StatusCode,
			"latency_ms", time.Since(start).Milliseconds())
	} else {
		h.log.Debug("SendGrid health check passed",
			"status_code", resp.StatusCode,
			"latency_ms", time.Since(start).Milliseconds())
	}

	result.Latency = time.Since(start)
	return result
}
