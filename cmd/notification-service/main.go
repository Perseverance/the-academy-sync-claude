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

	log.Printf("Notification Service starting in %s environment", cfg.Environment)
	log.Printf("Database URL configured: %t", cfg.DatabaseURL != "")
	log.Printf("Redis URL configured: %t", cfg.RedisURL != "")
	log.Printf("SMTP configured: %t", cfg.SMTPHost != "" && cfg.SMTPUsername != "")

	for {
		fmt.Printf("Notification Service is processing notifications in %s environment...\n", cfg.Environment)
		time.Sleep(30 * time.Second)
	}
}