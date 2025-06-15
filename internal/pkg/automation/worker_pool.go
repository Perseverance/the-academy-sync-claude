package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
)

// JobProcessor defines the interface for processing different types of jobs
type JobProcessor interface {
	ProcessJob(ctx context.Context, job *queue.Job) error
	CanProcess(jobType queue.JobType) bool
}

// WorkerPool manages a pool of workers that process jobs from Redis queue
type WorkerPool struct {
	redis       *redis.Client
	queueName   string
	processors  []JobProcessor
	workerCount int
	logger      *logger.Logger
	
	// Control channels
	ctx        context.Context
	cancel     context.CancelFunc
	workerWG   sync.WaitGroup
	
	// Statistics
	stats struct {
		mu            sync.RWMutex
		jobsProcessed int64
		jobsFailed    int64
		workersActive int32
	}
}

// WorkerPoolConfig configures the worker pool
type WorkerPoolConfig struct {
	RedisURL    string
	QueueName   string
	WorkerCount int
	PollInterval time.Duration
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(config WorkerPoolConfig, processors []JobProcessor, logger *logger.Logger) (*WorkerPool, error) {
	if config.QueueName == "" {
		config.QueueName = "jobs_queue"
	}
	if config.WorkerCount <= 0 {
		config.WorkerCount = 3
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 5 * time.Second
	}

	// Parse Redis URL and create client
	opts, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("Worker pool Redis connection established",
		"queue_name", config.QueueName,
		"worker_count", config.WorkerCount,
		"poll_interval", config.PollInterval)

	ctx, cancel = context.WithCancel(context.Background())

	return &WorkerPool{
		redis:       client,
		queueName:   config.QueueName,
		processors:  processors,
		workerCount: config.WorkerCount,
		logger:      logger.WithContext("component", "worker_pool"),
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	wp.logger.Info("Starting worker pool",
		"worker_count", wp.workerCount,
		"queue_name", wp.queueName)

	// Start workers
	for i := 0; i < wp.workerCount; i++ {
		wp.workerWG.Add(1)
		go wp.worker(i)
	}

	wp.logger.Info("Worker pool started successfully",
		"workers_active", wp.workerCount)
}

// Stop gracefully stops the worker pool
func (wp *WorkerPool) Stop() {
	wp.logger.Info("Stopping worker pool...")
	
	// Cancel context to signal workers to stop
	wp.cancel()
	
	// Wait for all workers to finish
	wp.workerWG.Wait()
	
	// Close Redis connection
	if err := wp.redis.Close(); err != nil {
		wp.logger.Error("Error closing Redis connection", "error", err)
	}
	
	wp.logger.Info("Worker pool stopped successfully")
}

// GetStats returns current worker pool statistics
func (wp *WorkerPool) GetStats() map[string]interface{} {
	wp.stats.mu.RLock()
	defer wp.stats.mu.RUnlock()
	
	return map[string]interface{}{
		"jobs_processed": wp.stats.jobsProcessed,
		"jobs_failed":    wp.stats.jobsFailed,
		"workers_active": wp.stats.workersActive,
		"worker_count":   wp.workerCount,
		"queue_name":     wp.queueName,
	}
}

// worker is the main worker loop that processes jobs
func (wp *WorkerPool) worker(workerID int) {
	defer wp.workerWG.Done()
	
	workerLogger := wp.logger.WithContext("worker_id", workerID)
	workerLogger.Info("Worker started")
	
	// Update active worker count
	wp.updateWorkerCount(1)
	defer wp.updateWorkerCount(-1)
	
	pollInterval := 5 * time.Second
	
	for {
		select {
		case <-wp.ctx.Done():
			workerLogger.Info("Worker stopping due to context cancellation")
			return
		default:
			// Try to get a job from the queue
			job, err := wp.dequeueJob(wp.ctx)
			if err != nil {
				if err == redis.Nil {
					// No jobs available, wait and try again
					time.Sleep(pollInterval)
					continue
				}
				
				workerLogger.Error("Failed to dequeue job", "error", err)
				time.Sleep(pollInterval)
				continue
			}
			
			if job == nil {
				// No job available
				time.Sleep(pollInterval)
				continue
			}
			
			// Process the job
			wp.processJob(workerLogger, job)
		}
	}
}

// dequeueJob removes and returns a job from the Redis queue
func (wp *WorkerPool) dequeueJob(ctx context.Context) (*queue.Job, error) {
	// Use BRPOP for blocking pop with timeout
	result, err := wp.redis.BRPop(ctx, 1*time.Second, wp.queueName).Result()
	if err != nil {
		if err == redis.Nil {
			// No job available within timeout
			return nil, nil
		}
		return nil, fmt.Errorf("failed to pop job from queue: %w", err)
	}
	
	if len(result) < 2 {
		return nil, fmt.Errorf("invalid BRPOP result format")
	}
	
	// result[0] is the queue name, result[1] is the job data
	jobData := result[1]
	
	// Deserialize job
	var job queue.Job
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, fmt.Errorf("failed to deserialize job: %w", err)
	}
	
	return &job, nil
}

// processJob processes a single job using the appropriate processor
func (wp *WorkerPool) processJob(logger *logger.Logger, job *queue.Job) {
	startTime := time.Now()
	
	logger.Info("Processing job",
		"job_id", job.ID,
		"job_type", job.Type,
		"user_id", job.UserID,
		"trace_id", job.TraceID)
	
	// Find a processor for this job type
	var processor JobProcessor
	for _, p := range wp.processors {
		if p.CanProcess(job.Type) {
			processor = p
			break
		}
	}
	
	if processor == nil {
		logger.Error("No processor found for job type",
			"job_id", job.ID,
			"job_type", job.Type,
			"trace_id", job.TraceID)
		wp.incrementFailedJobs()
		return
	}
	
	// Process the job
	if err := processor.ProcessJob(wp.ctx, job); err != nil {
		logger.Error("Job processing failed",
			"job_id", job.ID,
			"job_type", job.Type,
			"user_id", job.UserID,
			"error", err,
			"trace_id", job.TraceID,
			"duration", time.Since(startTime))
		wp.incrementFailedJobs()
		return
	}
	
	logger.Info("Job processed successfully",
		"job_id", job.ID,
		"job_type", job.Type,
		"user_id", job.UserID,
		"trace_id", job.TraceID,
		"duration", time.Since(startTime))
	
	wp.incrementProcessedJobs()
}

// updateWorkerCount updates the active worker count
func (wp *WorkerPool) updateWorkerCount(delta int32) {
	wp.stats.mu.Lock()
	wp.stats.workersActive += delta
	wp.stats.mu.Unlock()
}

// incrementProcessedJobs increments the processed jobs counter
func (wp *WorkerPool) incrementProcessedJobs() {
	wp.stats.mu.Lock()
	wp.stats.jobsProcessed++
	wp.stats.mu.Unlock()
}

// incrementFailedJobs increments the failed jobs counter
func (wp *WorkerPool) incrementFailedJobs() {
	wp.stats.mu.Lock()
	wp.stats.jobsFailed++
	wp.stats.mu.Unlock()
}

// ManualSyncProcessor implements JobProcessor for manual sync jobs
type ManualSyncProcessor struct {
	configService       ConfigServiceInterface
	tokenRefreshService *TokenRefreshService
	logger              *logger.Logger
}

// ConfigServiceInterface defines the interface for config service
type ConfigServiceInterface interface {
	GetProcessingConfigForUser(ctx context.Context, userID int) (*ProcessingConfig, error)
}

// NewManualSyncProcessor creates a new manual sync processor
func NewManualSyncProcessor(
	configService ConfigServiceInterface,
	tokenRefreshService *TokenRefreshService,
	logger *logger.Logger,
) *ManualSyncProcessor {
	return &ManualSyncProcessor{
		configService:       configService,
		tokenRefreshService: tokenRefreshService,
		logger:              logger.WithContext("component", "manual_sync_processor"),
	}
}

// CanProcess returns true if this processor can handle the job type
func (p *ManualSyncProcessor) CanProcess(jobType queue.JobType) bool {
	return jobType == queue.JobTypeManualSync
}

// ProcessJob processes a manual sync job with concurrent API operations
func (p *ManualSyncProcessor) ProcessJob(ctx context.Context, job *queue.Job) error {
	p.logger.Info("Processing manual sync job",
		"job_id", job.ID,
		"user_id", job.UserID,
		"trace_id", job.TraceID)

	// Get user configuration
	config, err := p.configService.GetProcessingConfigForUser(ctx, job.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user configuration: %w", err)
	}

	// Refresh tokens if needed
	if p.tokenRefreshService != nil {
		config, err = p.tokenRefreshService.RefreshTokensIfNeeded(ctx, config)
		if err != nil {
			return fmt.Errorf("failed to refresh tokens: %w", err)
		}
	}

	// Use goroutines and channels for concurrent API operations
	type apiResult struct {
		source string
		data   interface{}
		err    error
	}

	// Channel to collect results from concurrent API calls
	results := make(chan apiResult, 2)
	
	// Start concurrent API operations
	var activeOperations int

	// Fetch from Strava API in a goroutine
	if config.HasValidStravaToken() {
		activeOperations++
		go func() {
			defer func() {
				if r := recover(); r != nil {
					p.logger.Error("Panic in Strava API call", "panic", r, "job_id", job.ID)
					results <- apiResult{source: "strava", err: fmt.Errorf("panic in Strava API: %v", r)}
				}
			}()
			
			p.logger.Debug("Fetching data from Strava API", "job_id", job.ID, "user_id", job.UserID)
			
			// TODO: Implement actual Strava API call
			// For now, simulate API call
			time.Sleep(500 * time.Millisecond)
			
			// Simulate successful data fetch
			stravaData := map[string]interface{}{
				"activities": []string{"activity1", "activity2"},
				"athlete_id": config.StravaAthleteID,
			}
			
			results <- apiResult{source: "strava", data: stravaData}
		}()
	}

	// Validate Google Sheets access in a goroutine
	if config.HasValidGoogleToken() {
		activeOperations++
		go func() {
			defer func() {
				if r := recover(); r != nil {
					p.logger.Error("Panic in Google Sheets validation", "panic", r, "job_id", job.ID)
					results <- apiResult{source: "google", err: fmt.Errorf("panic in Google Sheets: %v", r)}
				}
			}()
			
			p.logger.Debug("Validating Google Sheets access", "job_id", job.ID, "user_id", job.UserID)
			
			// TODO: Implement actual Google Sheets validation
			// For now, simulate validation
			time.Sleep(300 * time.Millisecond)
			
			sheetsResult := map[string]interface{}{
				"spreadsheet_id": config.SpreadsheetID,
				"access_valid":   true,
			}
			
			results <- apiResult{source: "google", data: sheetsResult}
		}()
	}

	// Collect results from all concurrent operations
	var stravaData, googleData interface{}
	var errors []error

	for i := 0; i < activeOperations; i++ {
		select {
		case result := <-results:
			if result.err != nil {
				p.logger.Error("API operation failed",
					"source", result.source,
					"error", result.err,
					"job_id", job.ID)
				errors = append(errors, fmt.Errorf("%s: %w", result.source, result.err))
			} else {
				p.logger.Debug("API operation completed successfully",
					"source", result.source,
					"job_id", job.ID)
				switch result.source {
				case "strava":
					stravaData = result.data
				case "google":
					googleData = result.data
				}
			}
		case <-ctx.Done():
			return fmt.Errorf("job processing cancelled: %w", ctx.Err())
		case <-time.After(30 * time.Second):
			return fmt.Errorf("timeout waiting for API operations to complete")
		}
	}

	// Check if we had any critical errors
	if len(errors) > 0 {
		return fmt.Errorf("API operations failed: %v", errors)
	}

	// Process the collected data (this would normally write to Google Sheets)
	p.logger.Info("Processing collected data from APIs",
		"job_id", job.ID,
		"user_id", job.UserID,
		"has_strava_data", stravaData != nil,
		"has_google_access", googleData != nil)

	// TODO: Implement actual data processing and writing to Google Sheets
	// This is where you would:
	// 1. Transform Strava activity data
	// 2. Write to Google Sheets using the validated access
	// 3. Send notification emails if enabled

	p.logger.Info("Manual sync job completed successfully",
		"job_id", job.ID,
		"user_id", job.UserID,
		"trace_id", job.TraceID,
		"processed_operations", activeOperations)

	return nil
}