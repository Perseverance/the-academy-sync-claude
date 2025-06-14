package main

import (
	"fmt"
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
		return
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

	for {
		log.Debug("Processing notification queue", "environment", cfg.Environment)
		time.Sleep(30 * time.Second)
	}
}