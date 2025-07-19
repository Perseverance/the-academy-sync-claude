package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/Perseverance/the-academy-sync-claude/cmd/notification-service/internal"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/config"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/health"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/retry"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/sendgrid"
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

	// SendGrid health check
	if cfg.SendGridAPIKey != "" {
		err := retry.WithExponentialBackoff(ctx, retry.CriticalConfig(), log, "sendgrid_health_check", func() error {
			result := healthChecker.CheckSendGrid(ctx, cfg.SendGridAPIKey)
			if !result.IsHealthy() {
				return fmt.Errorf("SendGrid health check failed: %w", result.Error)
			}
			return nil
		})

		if err != nil {
			if cfg.FailFastEnabled {
				log.Critical("SendGrid dependency check failed with fail-fast enabled",
					"error", err.Error())
				return fmt.Errorf("SendGrid dependency check failed: %w", err)
			} else {
				log.Warn("SendGrid dependency check failed - notification service will run with limited functionality",
					"error", err.Error())
				// Continue operation with reduced functionality when fail-fast is disabled
			}
		} else {
			log.Info("SendGrid health check passed - email functionality is available")
		}
	}

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
		"sendgrid_configured", cfg.SendGridAPIKey != "",
		"from_email", cfg.FromEmail)

	// Dependency Health Check - US046 Fail Fast Mechanism
	// Validate critical dependencies before starting processing loop
	if err := performStartupHealthChecks(cfg, log); err != nil {
		log.Critical("Startup dependency health checks failed - notification service cannot continue",
			"error", err.Error())
		os.Exit(2) // Exit code 2 indicates dependency failure
	}

	// Initialize SendGrid client if configured
	var sendgridClient *sendgrid.Client
	if cfg.SendGridAPIKey != "" && cfg.FromEmail != "" {
		sendgridClient = sendgrid.NewClient(cfg.SendGridAPIKey, cfg.FromEmail)
		log.Info("SendGrid client initialized",
			"from_email", cfg.FromEmail)
	} else {
		log.Warn("SendGrid not configured - email functionality will be disabled")
	}

	// Start HTTP server for health checks (required by Cloud Run)
	go startHealthServer(log)

	// Initialize Redis queue client if configured
	if cfg.RedisURL != "" {
		log.Info("Initializing Redis queue client",
			"redis_url", cfg.RedisURL)

		queueClient, err := queue.NewClient(cfg.RedisURL, "notification_queue", log)
		if err != nil {
			log.Critical("Failed to initialize Redis queue client", "error", err)
			os.Exit(1)
		}
		defer queueClient.Close()

		// Create notification service
		notificationService := internal.NewNotificationService(sendgridClient, log)

		// Create and start worker
		worker := internal.NewWorker(queueClient, notificationService, log)
		
		// Start processing in a goroutine
		ctx := context.Background()
		go func() {
			log.Info("Starting notification worker")
			worker.ProcessJobs(ctx)
		}()

		log.Info("Notification service is running and processing queue")
		
		// Keep the main goroutine alive
		select {}
	} else {
		log.Warn("Redis not configured - notification service running in standby mode")
		// Keep the service alive for health checks
		select {}
	}
}

// startHealthServer starts an HTTP server for health checks required by Cloud Run
func startHealthServer(log *logger.Logger) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Notification Service is running\n")
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK\n")
	})

	log.Info("Starting HTTP server for health checks", "port", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Error("Failed to start HTTP server", "error", err)
		os.Exit(1)
	}
}
