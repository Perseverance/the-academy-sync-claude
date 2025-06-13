package main

import (
	"fmt"
	"log"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/config"
)

func main() {
	// Load configuration using hybrid loading strategy
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Automation Engine starting in %s environment", cfg.Environment)
	log.Printf("Database URL configured: %t", cfg.DatabaseURL != "")
	log.Printf("Redis URL configured: %t", cfg.RedisURL != "")
	log.Printf("Strava OAuth configured: %t", cfg.StravaClientID != "" && cfg.StravaClientSecret != "")

	for {
		fmt.Printf("Automation Engine is processing jobs in %s environment...\n", cfg.Environment)
		time.Sleep(30 * time.Second)
	}
}