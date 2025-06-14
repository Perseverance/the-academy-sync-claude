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
// This function implements the US046 fail-fast mechanism for automation engine dependencies
func performStartupHealthChecks(cfg *config.Config, log *logger.Logger) error {
	log.Info("Starting dependency health checks")
	
	// Create health checker
	healthChecker := health.NewHealthChecker(log)
	
	// Create context with timeout for all health checks
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// For automation engine, database connectivity is critical for job processing
	if cfg.DatabaseURL != "" {
		err := retry.WithExponentialBackoff(ctx, retry.CriticalConfig(), log, "database_health_check", func() error {
			result := healthChecker.CheckDatabaseConnection(ctx, cfg.DatabaseURL)
			if !result.IsHealthy() {
				return fmt.Errorf("database health check failed: %w", result.Error)
			}
			return nil
		})
		
		if err != nil {
			log.Critical("Critical dependency failed: Database connection unavailable after retries", 
				"error", err.Error())
			return fmt.Errorf("database dependency check failed: %w", err)
		}
	}
	
	// TODO: Add Redis health check when Redis connectivity is implemented
	// if cfg.RedisURL != "" {
	//     // Redis health check logic here
	// }
	
	log.Info("All critical dependency health checks passed successfully")
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
	log := logger.New("automation-engine")

	log.Info("Automation Engine starting", 
		"environment", cfg.Environment,
		"log_level", cfg.LogLevel)
	log.Info("Configuration status", 
		"database_configured", cfg.DatabaseURL != "",
		"redis_configured", cfg.RedisURL != "",
		"strava_oauth_configured", cfg.StravaClientID != "" && cfg.StravaClientSecret != "")

	// Dependency Health Check - US046 Fail Fast Mechanism
	// Validate critical dependencies before starting processing loop
	if err := performStartupHealthChecks(cfg, log); err != nil {
		log.Critical("Startup dependency health checks failed - automation engine cannot continue", 
			"error", err.Error())
		os.Exit(2) // Exit code 2 indicates dependency failure
	}

	for {
		log.Debug("Processing job queue", "environment", cfg.Environment)
		time.Sleep(30 * time.Second)
	}
}