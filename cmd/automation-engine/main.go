package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/config"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

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

	for {
		log.Debug("Processing job queue", "environment", cfg.Environment)
		time.Sleep(30 * time.Second)
	}
}