package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
	
	// Redis health check (if Redis is configured)
	if cfg.RedisURL != "" {
		log.Info("Performing Redis health check", "redis_url", cfg.RedisURL)
		
		err := retry.WithExponentialBackoff(ctx, retry.CriticalConfig(), log, "redis_health_check", func() error {
			// Create a temporary queue client for health check
			tempQueueClient, err := queue.NewClient(cfg.RedisURL, "health_check_queue", log)
			if err != nil {
				return fmt.Errorf("redis connection failed: %w", err)
			}
			defer tempQueueClient.Close()
			
			// Perform health check
			if err := tempQueueClient.HealthCheck(ctx); err != nil {
				return fmt.Errorf("redis health check failed: %w", err)
			}
			
			return nil
		})
		
		if err != nil {
			log.Critical("Critical dependency failed: Redis connection unavailable after retries", 
				"error", err.Error(),
				"redis_url", cfg.RedisURL)
			return fmt.Errorf("redis dependency check failed: %w", err)
		}
		
		log.Info("Redis health check passed successfully", "redis_url", cfg.RedisURL)
	} else {
		log.Warn("Redis not configured - queue-based sync functionality will be unavailable")
	}
	
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

	log.Info("Automation engine initialized successfully",
		"oauth_configured", cfg.StravaClientID != "" && cfg.GoogleClientID != "",
		"max_workers", cfg.MaxWorkers)

	// Initialize Redis queue client if configured
	if cfg.RedisURL != "" {
		log.Info("Redis configured - starting worker pool for job processing",
			"redis_url", cfg.RedisURL,
			"max_workers", cfg.MaxWorkers)
		
		// Initialize queue client
		queueClient, err := queue.NewClient(cfg.RedisURL, "jobs_queue", log)
		if err != nil {
			log.Critical("Failed to initialize Redis queue client", "error", err)
			os.Exit(1)
		}
		defer queueClient.Close()
		
		// Start worker pool
		startWorkerPool(queueClient, worker, cfg.MaxWorkers, log)
	} else {
		log.Warn("Redis not configured - running in test mode with single user processing")
		// Fall back to test processing for development
		runTestMode(worker, log)
	}
}

// startWorkerPool starts a pool of workers to process jobs from the queue
func startWorkerPool(queueClient *queue.Client, worker *processing.Worker, maxWorkers int, log *logger.Logger) {
	log.Info("Starting worker pool",
		"max_workers", maxWorkers,
		"queue", "jobs_queue")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Channel for job distribution to workers
	jobs := make(chan *queue.Job, maxWorkers*2) // Buffer to prevent blocking

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			runWorker(ctx, workerID, jobs, queueClient, worker, log)
		}(i + 1)
	}

	// Start job distributor
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(jobs)
		runJobDistributor(ctx, queueClient, jobs, log)
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Info("Shutdown signal received, stopping worker pool")

	cancel() // Signal all workers to stop
	wg.Wait() // Wait for all workers to finish

	log.Info("Worker pool stopped gracefully")
}

// runWorker processes jobs from the jobs channel
func runWorker(ctx context.Context, workerID int, jobs <-chan *queue.Job, queueClient *queue.Client, worker *processing.Worker, log *logger.Logger) {
	workerLog := log.WithContext("worker_id", workerID)
	workerLog.Info("Worker started")

	for {
		select {
		case <-ctx.Done():
			workerLog.Info("Worker shutting down")
			return
		case job, ok := <-jobs:
			if !ok {
				workerLog.Info("Jobs channel closed, worker stopping")
				return
			}

			processJob(ctx, workerID, job, queueClient, worker, workerLog)
		}
	}
}

// processJob processes a single job
func processJob(ctx context.Context, workerID int, job *queue.Job, queueClient *queue.Client, worker *processing.Worker, log *logger.Logger) {
	startTime := time.Now()
	
	log.Info("Processing job",
		"job_id", job.ID,
		"job_type", job.Type,
		"user_id", job.UserID,
		"trace_id", job.TraceID,
		"age_seconds", time.Since(job.CreatedAt).Seconds())

	// Ensure we release the processing lock when done
	defer func() {
		if err := queueClient.ReleaseUserProcessingLock(ctx, job.UserID); err != nil {
			log.Error("Failed to release user processing lock",
				"error", err,
				"user_id", job.UserID,
				"job_id", job.ID)
		} else {
			log.Debug("Released user processing lock",
				"user_id", job.UserID,
				"job_id", job.ID)
		}
	}()

	// Create context with timeout for job processing
	jobCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// Process based on job type
	var result *processing.ProcessingResult
	switch job.Type {
	case queue.JobTypeManualSync, queue.JobTypeScheduledSync:
		result = worker.ProcessUser(jobCtx, job.UserID)
	default:
		log.Error("Unknown job type",
			"job_id", job.ID,
			"job_type", job.Type,
			"user_id", job.UserID)
		return
	}

	duration := time.Since(startTime)

	if result.Success {
		log.Info("Job processed successfully",
			"job_id", job.ID,
			"job_type", job.Type,
			"user_id", job.UserID,
			"trace_id", job.TraceID,
			"processing_duration_ms", duration.Milliseconds(),
			"activities_count", result.ActivitiesCount)
	} else {
		log.Error("Job processing failed",
			"job_id", job.ID,
			"job_type", job.Type,
			"user_id", job.UserID,
			"trace_id", job.TraceID,
			"processing_duration_ms", duration.Milliseconds(),
			"error", result.Error,
			"error_type", result.ErrorType,
			"requires_reauth", result.RequiresReauth)
	}
}

// runJobDistributor fetches jobs from the queue and distributes them to workers
func runJobDistributor(ctx context.Context, queueClient *queue.Client, jobs chan<- *queue.Job, log *logger.Logger) {
	log.Info("Job distributor started")

	for {
		select {
		case <-ctx.Done():
			log.Info("Job distributor shutting down")
			return
		default:
			// Try to dequeue a job
			job, err := queueClient.DequeueJob(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					log.Info("Context cancelled, job distributor stopping")
					return
				}
				log.Error("Failed to dequeue job", "error", err)
				time.Sleep(5 * time.Second) // Wait before retrying
				continue
			}

			if job == nil {
				// No job available, continue polling
				continue
			}

			// Try to send job to worker
			select {
			case jobs <- job:
				log.Debug("Job distributed to worker",
					"job_id", job.ID,
					"job_type", job.Type,
					"user_id", job.UserID)
			case <-ctx.Done():
				log.Info("Context cancelled while distributing job")
				return
			}
		}
	}
}

// runTestMode runs the engine in test mode for development
func runTestMode(worker *processing.Worker, log *logger.Logger) {
	log.Info("Starting automation engine in test mode - will process user ID 1 every minute", 
		"note", "Production mode requires Redis configuration")
	
	testUserID := 1
	cycleCount := 0
	
	for {
		cycleCount++
		cycleStartTime := time.Now()
		
		// Create context for this processing cycle
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		
		log.Info("🔄 Starting test automation processing cycle",
			"test_user_id", testUserID,
			"cycle_number", cycleCount,
			"timeout_minutes", 5,
			"note", "This is a development test - production will use job queue")
		
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
		log.Debug("💤 Automation processing cycle completed, waiting for next cycle",
			"cycle_number", cycleCount,
			"cycle_duration_ms", cycleDuration.Milliseconds(),
			"next_cycle_at", time.Now().Add(60*time.Second).Format(time.RFC3339),
			"wait_seconds", 60)
		time.Sleep(60 * time.Second) // Process every minute for testing
	}
}