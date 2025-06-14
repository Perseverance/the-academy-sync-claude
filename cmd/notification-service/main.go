package main

import (
	"context"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/config"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/health"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/retry"
)

// performStartupHealthChecks validates critical dependencies and fails fast if any are unavailable
// This function implements the US046 fail-fast mechanism for notification service dependencies
func performStartupHealthChecks(cfg *config.Config, log *logger.Logger) error {
	log.Info("Starting dependency health checks")
	
	// Create health checker
	healthChecker := health.NewHealthChecker(log)
	
	// Create context with timeout for all health checks
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// For notification service, database connectivity is optional but recommended
	// Only enforce it if explicitly configured
	if cfg.DatabaseURL != "" {
		err := retry.WithExponentialBackoff(ctx, retry.CriticalConfig(), log, "database_health_check", func() error {
			result := healthChecker.CheckDatabaseConnection(ctx, cfg.DatabaseURL)
			if !result.IsHealthy() {
				return fmt.Errorf("database health check failed: %w", result.Error)
			}
			return nil
		})
		
		if err != nil {
			if cfg.FailFastEnabled {
				log.Critical("Database dependency check failed with fail-fast enabled", 
					"error", err.Error())
				return fmt.Errorf("database dependency check failed: %w", err)
			} else {
				log.Warn("Database dependency check failed - notification service will run with limited functionality", 
					"error", err.Error())
				// Continue operation with reduced functionality when fail-fast is disabled
			}
		}
	}
	
	// TODO: Add Redis health check when Redis connectivity is implemented
	// if cfg.RedisURL != "" {
	//     // Redis health check logic here
	// }
	
	// TODO: Add SMTP health check when email functionality is implemented
	// if cfg.SMTPHost != "" {
	//     // SMTP connectivity check logic here
	// }
	
	log.Info("Dependency health checks completed")
	return nil
}

func main() {
	// Load configuration using hybrid loading strategy
	cfg, err := config.Load()
	if err != nil {
		// Use fallback logging before structured logger is available
		fmt.Printf("ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	log := logger.New("notification-service")

	log.Info("Notification Service starting", 
		"environment", cfg.Environment,
		"log_level", cfg.LogLevel)
	log.Info("Configuration status", 
		"database_configured", cfg.DatabaseURL != "",
		"redis_configured", cfg.RedisURL != "",
		"smtp_configured", cfg.SMTPHost != "" && cfg.SMTPUsername != "")

	// Dependency Health Check - US046 Fail Fast Mechanism
	// Validate critical dependencies before starting processing loop
	if err := performStartupHealthChecks(cfg, log); err != nil {
		log.Critical("Startup dependency health checks failed - notification service cannot continue", 
			"error", err.Error())
		os.Exit(2) // Exit code 2 indicates dependency failure
	}

	for {
		log.Debug("Processing notification queue", "environment", cfg.Environment)
		time.Sleep(30 * time.Second)
	}
}