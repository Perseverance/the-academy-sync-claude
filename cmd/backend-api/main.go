package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/config"
)

func main() {
	// Load configuration using hybrid loading strategy
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Backend API starting in %s environment on port %s", cfg.Environment, cfg.Port)
	log.Printf("Database URL configured: %t", cfg.DatabaseURL != "")
	log.Printf("Redis URL configured: %t", cfg.RedisURL != "")
	log.Printf("Google OAuth configured: %t", cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Backend API Service is running in %s environment!", cfg.Environment)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "healthy", "environment": "%s", "service": "backend-api"}`, cfg.Environment)
	})

	log.Printf("Backend API starting on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}