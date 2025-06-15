package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/Perseverance/the-academy-sync-claude/cmd/automation-engine/internal/processing"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/automation"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/config"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/health"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
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
	
	// Validate critical dependencies - DATABASE_URL is required for automation engine
	if cfg.DatabaseURL == "" {
		log.Critical("Critical dependency validation failed: DATABASE_URL not configured")
		return fmt.Errorf("DATABASE_URL is required but not configured")
	}

	// For automation engine, database connectivity is critical for job processing
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

	// Initialize database connection
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Critical("Failed to open database connection", "error", err.Error())
		os.Exit(3)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Critical("Failed to ping database", "error", err.Error())
		os.Exit(3)
	}

	log.Info("Database connection established successfully")

	// Initialize encryption service for token handling
	encryptionService := auth.NewEncryptionService(cfg.EncryptionSecret)

	// Initialize repositories and services
	userRepository := database.NewUserRepository(db, encryptionService)
	configService := automation.NewConfigService(userRepository, log)

	// Initialize Redis queue client (optional)
	var queueClient *queue.Client
	if cfg.RedisURL != "" {
		var err error
		queueClient, err = queue.NewClient(cfg.RedisURL, log)
		if err != nil {
			log.Critical("Failed to initialize Redis queue client", "error", err)
			os.Exit(3)
		}
		log.Info("Redis queue client initialized successfully")
	} else {
		log.Info("Redis URL not configured - running in test mode with periodic processing")
	}

	// Initialize processing worker
	worker := processing.NewWorker(
		configService,
		cfg.StravaClientID,
		cfg.StravaClientSecret,
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		"", // GoogleRedirectURL not needed for server-side token refresh
		log,
	)

	log.Info("Automation engine initialized successfully, starting processing loop",
		"oauth_configured", cfg.StravaClientID != "" && cfg.GoogleClientID != "",
		"redis_configured", queueClient != nil)

	if queueClient != nil {
		// Redis job queue mode
		log.Info("🚀 Starting Redis job queue processing mode")
		startJobQueueProcessing(queueClient, worker, log, cfg)
	} else {
		// Test mode with periodic processing
		log.Info("🧪 Starting test mode with periodic processing")
		startTestModeProcessing(worker, log, cfg)
	}
}

// startJobQueueProcessing processes jobs from Redis queue using worker pool pattern
func startJobQueueProcessing(queueClient *queue.Client, worker *processing.Worker, log *logger.Logger, cfg *config.Config) {
	// Use centralized configuration for worker count
	maxWorkers := cfg.MaxWorkers
	
	// Create unbuffered channel for job distribution
	jobChan := make(chan *queue.Job)
	
	log.Info("🚀 Starting job queue processing with worker pool",
		"max_workers", maxWorkers,
		"queue_name", queue.JobsQueueName)
	
	// Start worker pool
	for i := 0; i < maxWorkers; i++ {
		go func(workerID int) {
			log.Debug("🔧 Worker started",
				"worker_id", workerID)
			
			for job := range jobChan {
				processJob(job, worker, log, workerID)
			}
			
			log.Debug("🔧 Worker stopped",
				"worker_id", workerID)
		}(i)
	}
	
	// Job dequeue goroutine - feeds jobs to worker pool
	go func() {
		jobCount := 0
		
		for {
			jobCount++
			log.Debug("📥 Waiting for job from Redis queue",
				"job_number", jobCount,
				"queue_name", queue.JobsQueueName,
				"active_workers", maxWorkers)
			
			// Create context for dequeue operation
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			
			// Dequeue job (blocking operation)
			job, err := queueClient.DequeueJob(ctx)
			cancel()
			
			if err != nil {
				log.Error("Failed to dequeue job from Redis",
					"error", err,
					"job_number", jobCount)
				time.Sleep(5 * time.Second) // Wait before retrying
				continue
			}
			
			if job == nil {
				// No job available (timeout) - this is normal
				log.Debug("No job available, continuing to wait",
					"job_number", jobCount)
				continue
			}
			
			log.Debug("📤 Job dequeued, sending to worker pool",
				"job_number", jobCount,
				"user_id", job.UserID,
				"trace_id", job.TraceID,
				"trigger_type", job.TriggerType,
				"job_age_seconds", time.Since(job.CreatedAt).Seconds())
			
			// Send job to available worker (blocks until worker is free)
			jobChan <- job
		}
	}()
	
	// Keep main goroutine alive - in production this would handle shutdown signals
	select {}
}

// processJob handles individual job processing within a worker
func processJob(job *queue.Job, worker *processing.Worker, log *logger.Logger, workerID int) {
	jobStartTime := time.Now()
	
	log.Info("🎯 Worker processing job",
		"worker_id", workerID,
		"user_id", job.UserID,
		"trace_id", job.TraceID,
		"trigger_type", job.TriggerType,
		"job_created_at", job.CreatedAt.Format(time.RFC3339),
		"job_age_seconds", time.Since(job.CreatedAt).Seconds())
	
	// Create context for job processing with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	
	// Process user with job context
	result := worker.ProcessUser(ctx, job.UserID)
	
	jobDuration := time.Since(jobStartTime)
	totalJobAge := time.Since(job.CreatedAt)
	
	if result.Success {
		log.Info("✅ Worker completed job successfully",
			"worker_id", workerID,
			"user_id", job.UserID,
			"trace_id", job.TraceID,
			"trigger_type", job.TriggerType,
			"job_results", map[string]interface{}{
				"activities_count":        result.ActivitiesCount,
				"user_processing_time_ms": result.ProcessingTime.Milliseconds(),
				"total_job_time_ms":       jobDuration.Milliseconds(),
				"total_job_age_ms":        totalJobAge.Milliseconds(),
				"success":                 true,
			})
	} else {
		log.Warn("⚠️ Worker job processing failed",
			"worker_id", workerID,
			"user_id", job.UserID,
			"trace_id", job.TraceID,
			"trigger_type", job.TriggerType,
			"job_results", map[string]interface{}{
				"error":                   result.Error,
				"error_type":              result.ErrorType,
				"requires_reauth":         result.RequiresReauth,
				"user_processing_time_ms": result.ProcessingTime.Milliseconds(),
				"total_job_time_ms":       jobDuration.Milliseconds(),
				"total_job_age_ms":        totalJobAge.Milliseconds(),
				"success":                 false,
			})
	}
}

// startTestModeProcessing runs periodic test processing
func startTestModeProcessing(worker *processing.Worker, log *logger.Logger, cfg *config.Config) {
	cycleCount := 0
	for {
		cycleCount++
		cycleStartTime := time.Now()
		
		log.Debug("🔄 Starting automation test processing cycle",
			"cycle_number", cycleCount,
			"cycle_start_time", cycleStartTime.Format(time.RFC3339),
			"environment", cfg.Environment,
			"next_cycle_in_seconds", 60)
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		
		// Test processing with user ID 1 (if exists)
		testUserID := 1
		
		log.Debug("🧪 Starting test processing for development user",
			"test_user_id", testUserID,
			"cycle_number", cycleCount,
			"timeout_minutes", 5,
			"note", "This is a development test - production will use Redis job queue")
		
		result := worker.ProcessUser(ctx, testUserID)
		
		cycleDuration := time.Since(cycleStartTime)
		
		if result.Success {
			log.Info("✅ Test processing cycle completed successfully",
				"cycle_number", cycleCount,
				"user_id", testUserID,
				"cycle_results", map[string]interface{}{
					"activities_count":        result.ActivitiesCount,
					"user_processing_time_ms": result.ProcessingTime.Milliseconds(),
					"total_cycle_time_ms":     cycleDuration.Milliseconds(),
					"success":                 true,
				})
		} else {
			log.Warn("⚠️ Test processing cycle failed",
				"cycle_number", cycleCount,
				"user_id", testUserID,
				"cycle_results", map[string]interface{}{
					"error":                   result.Error,
					"error_type":              result.ErrorType,
					"requires_reauth":         result.RequiresReauth,
					"user_processing_time_ms": result.ProcessingTime.Milliseconds(),
					"total_cycle_time_ms":     cycleDuration.Milliseconds(),
					"success":                 false,
				},
				"troubleshooting", map[string]interface{}{
					"check_user_exists":      "Verify user ID 1 exists in database",
					"check_oauth_tokens":     "Verify user has valid OAuth tokens",
					"check_spreadsheet_id":   "Verify user has configured spreadsheet ID",
					"check_oauth_credentials": "Verify app OAuth credentials are configured",
				})
		}
		
		cancel()
		
		// Wait before next cycle
		log.Debug("💤 Test processing cycle completed, waiting for next cycle",
			"cycle_number", cycleCount,
			"cycle_duration_ms", cycleDuration.Milliseconds(),
			"next_cycle_at", time.Now().Add(60*time.Second).Format(time.RFC3339),
			"wait_seconds", 60)
		time.Sleep(60 * time.Second) // Process every minute for testing
	}
}