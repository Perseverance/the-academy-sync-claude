package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// JobType represents the type of job
type JobType string

const (
	// JobTypeManualSync represents a manual sync job triggered by user
	JobTypeManualSync JobType = "manual_sync"
	// JobTypeScheduledSync represents a scheduled sync job
	JobTypeScheduledSync JobType = "scheduled_sync"
)

// Job represents a job in the queue
type Job struct {
	ID        string                 `json:"id"`
	Type      JobType                `json:"type"`
	UserID    int                    `json:"user_id"`
	Data      map[string]interface{} `json:"data,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	TraceID   string                 `json:"trace_id"`
}

// Client provides Redis queue operations
type Client struct {
	redis     *redis.Client
	queueName string
	logger    *logger.Logger
}

// NewClient creates a new queue client
func NewClient(redisURL string, queueName string, logger *logger.Logger) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error("Failed to parse Redis URL", "error", err)
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Set connection pool options for better performance
	opts.PoolSize = 10
	opts.MinIdleConns = 5
	opts.MaxRetries = 3
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.Error("Failed to connect to Redis", "error", err, "url", redisURL)
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("Successfully connected to Redis queue",
		"queue_name", queueName,
		"pool_size", opts.PoolSize)

	return &Client{
		redis:     client,
		queueName: queueName,
		logger:    logger.WithContext("component", "queue_client"),
	}, nil
}

// EnqueueJob adds a job to the queue
func (c *Client) EnqueueJob(ctx context.Context, jobType JobType, userID int, data map[string]interface{}) (*Job, error) {
	job := &Job{
		ID:        uuid.New().String(),
		Type:      jobType,
		UserID:    userID,
		Data:      data,
		CreatedAt: time.Now().UTC(),
		TraceID:   uuid.New().String(),
	}

	c.logger.Debug("Enqueueing job",
		"job_id", job.ID,
		"job_type", jobType,
		"user_id", userID,
		"trace_id", job.TraceID,
		"queue", c.queueName)

	// Serialize job to JSON
	jobData, err := json.Marshal(job)
	if err != nil {
		c.logger.Error("Failed to marshal job",
			"error", err,
			"job_id", job.ID,
			"user_id", userID)
		return nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	// Push to Redis queue (LPUSH for FIFO with BRPOP)
	if err := c.redis.LPush(ctx, c.queueName, jobData).Err(); err != nil {
		c.logger.Error("Failed to enqueue job",
			"error", err,
			"job_id", job.ID,
			"user_id", userID,
			"queue", c.queueName)
		return nil, fmt.Errorf("failed to enqueue job: %w", err)
	}

	c.logger.Info("Successfully enqueued job",
		"job_id", job.ID,
		"job_type", jobType,
		"user_id", userID,
		"trace_id", job.TraceID,
		"queue", c.queueName)

	return job, nil
}

// DequeueJob blocks until a job is available and returns it
func (c *Client) DequeueJob(ctx context.Context) (*Job, error) {
	c.logger.Debug("Waiting for job from queue", "queue", c.queueName)

	// BRPOP blocks until an item is available or timeout
	result, err := c.redis.BRPop(ctx, 0, c.queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No job available
		}
		c.logger.Error("Failed to dequeue job",
			"error", err,
			"queue", c.queueName)
		return nil, fmt.Errorf("failed to dequeue job: %w", err)
	}

	// result[0] is the queue name, result[1] is the data
	if len(result) != 2 {
		c.logger.Error("Unexpected BRPOP result format",
			"result_length", len(result),
			"queue", c.queueName)
		return nil, fmt.Errorf("unexpected BRPOP result format")
	}

	var job Job
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		c.logger.Error("Failed to unmarshal job",
			"error", err,
			"raw_data", result[1])
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	c.logger.Info("Successfully dequeued job",
		"job_id", job.ID,
		"job_type", job.Type,
		"user_id", job.UserID,
		"trace_id", job.TraceID,
		"queue", c.queueName,
		"age_seconds", time.Since(job.CreatedAt).Seconds())

	return &job, nil
}

// GetQueueLength returns the current length of the queue
func (c *Client) GetQueueLength(ctx context.Context) (int64, error) {
	length, err := c.redis.LLen(ctx, c.queueName).Result()
	if err != nil {
		c.logger.Error("Failed to get queue length",
			"error", err,
			"queue", c.queueName)
		return 0, fmt.Errorf("failed to get queue length: %w", err)
	}

	c.logger.Debug("Queue length retrieved",
		"queue", c.queueName,
		"length", length)

	return length, nil
}

// HealthCheck verifies Redis connectivity
func (c *Client) HealthCheck(ctx context.Context) error {
	if err := c.redis.Ping(ctx).Err(); err != nil {
		c.logger.Error("Redis health check failed", "error", err)
		return fmt.Errorf("redis health check failed: %w", err)
	}
	return nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	c.logger.Info("Closing Redis connection")
	return c.redis.Close()
}

// IsConnectionError checks if an error is a Redis connection error
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common Redis connection error patterns
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "connect: connection refused")
}