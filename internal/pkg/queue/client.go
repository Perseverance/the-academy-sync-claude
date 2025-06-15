package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

const (
	// JobsQueueName is the Redis list name for job queue
	JobsQueueName = "jobs_queue"
	
	// JobTimeout is the maximum time to wait for blocking operations
	JobTimeout = 30 * time.Second
)

// Job represents a job to be processed by the automation engine
type Job struct {
	UserID        int       `json:"user_id"`
	TraceID       string    `json:"trace_id"`
	TriggerType   string    `json:"trigger_type"`
	CreatedAt     time.Time `json:"created_at"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

// Client provides Redis-based job queue operations
type Client struct {
	redis  *redis.Client
	logger *logger.Logger
}

// NewClient creates a new Redis queue client
func NewClient(redisURL string, logger *logger.Logger) (*Client, error) {
	// Parse Redis URL
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Configure connection pooling and timeouts
	opt.PoolSize = 10
	opt.MinIdleConns = 3
	opt.ConnMaxIdleTime = 5 * time.Minute
	opt.ConnMaxLifetime = 30 * time.Minute

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{
		redis:  client,
		logger: logger.WithContext("component", "queue_client"),
	}, nil
}

// EnqueueJob adds a job to the queue
func (c *Client) EnqueueJob(ctx context.Context, userID int, triggerType string) (*Job, error) {
	// Generate unique trace ID
	traceID := uuid.New().String()

	// Create job
	job := &Job{
		UserID:         userID,
		TraceID:        traceID,
		TriggerType:    triggerType,
		CreatedAt:      time.Now(),
		TimeoutSeconds: 300, // 5 minutes default timeout
	}

	// Serialize job to JSON
	jobData, err := json.Marshal(job)
	if err != nil {
		c.logger.Error("Failed to marshal job to JSON",
			"error", err,
			"user_id", userID,
			"trace_id", traceID,
			"trigger_type", triggerType)
		return nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	// Add to Redis queue (LPUSH for FIFO when using BRPOP)
	err = c.redis.LPush(ctx, JobsQueueName, jobData).Err()
	if err != nil {
		c.logger.Error("Failed to enqueue job to Redis",
			"error", err,
			"user_id", userID,
			"trace_id", traceID,
			"trigger_type", triggerType,
			"queue_name", JobsQueueName)
		return nil, fmt.Errorf("failed to enqueue job: %w", err)
	}

	c.logger.Info("Successfully enqueued job",
		"user_id", userID,
		"trace_id", traceID,
		"trigger_type", triggerType,
		"queue_name", JobsQueueName,
		"created_at", job.CreatedAt.Format(time.RFC3339))

	return job, nil
}

// DequeueJob removes and returns a job from the queue (blocking operation)
func (c *Client) DequeueJob(ctx context.Context) (*Job, error) {
	// Use BRPOP for blocking right pop (FIFO order)
	result, err := c.redis.BRPop(ctx, JobTimeout, JobsQueueName).Result()
	if err != nil {
		if err == redis.Nil {
			// No job available (timeout)
			return nil, nil
		}
		c.logger.Error("Failed to dequeue job from Redis",
			"error", err,
			"queue_name", JobsQueueName)
		return nil, fmt.Errorf("failed to dequeue job: %w", err)
	}

	// BRPOP returns [queue_name, data]
	if len(result) != 2 {
		c.logger.Error("Unexpected BRPOP result format",
			"result_length", len(result),
			"queue_name", JobsQueueName)
		return nil, fmt.Errorf("unexpected Redis BRPOP result format")
	}

	jobData := result[1]

	// Deserialize job from JSON
	var job Job
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		c.logger.Error("Failed to unmarshal job from JSON",
			"error", err,
			"job_data", jobData,
			"queue_name", JobsQueueName)
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	c.logger.Info("Successfully dequeued job",
		"user_id", job.UserID,
		"trace_id", job.TraceID,
		"trigger_type", job.TriggerType,
		"queue_name", JobsQueueName,
		"created_at", job.CreatedAt.Format(time.RFC3339),
		"age_seconds", time.Since(job.CreatedAt).Seconds())

	return &job, nil
}

// GetQueueLength returns the current number of jobs in the queue
func (c *Client) GetQueueLength(ctx context.Context) (int64, error) {
	length, err := c.redis.LLen(ctx, JobsQueueName).Result()
	if err != nil {
		c.logger.Error("Failed to get queue length",
			"error", err,
			"queue_name", JobsQueueName)
		return 0, fmt.Errorf("failed to get queue length: %w", err)
	}
	return length, nil
}

// HealthCheck verifies Redis connectivity
func (c *Client) HealthCheck(ctx context.Context) error {
	err := c.redis.Ping(ctx).Err()
	if err != nil {
		c.logger.Error("Redis health check failed",
			"error", err)
		return fmt.Errorf("Redis health check failed: %w", err)
	}
	return nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	if c.redis != nil {
		return c.redis.Close()
	}
	return nil
}

// GenerateTraceID generates a new unique trace ID
func GenerateTraceID() string {
	return uuid.New().String()
}