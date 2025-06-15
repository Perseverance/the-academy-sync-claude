package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// JobType represents the type of job to be processed
type JobType string

const (
	// JobTypeManualSync represents a manual sync job triggered by user request
	JobTypeManualSync JobType = "manual_sync"
)

// Job represents a job to be processed by the automation engine
type Job struct {
	ID        string                 `json:"id"`
	Type      JobType                `json:"type"`
	UserID    int                    `json:"user_id"`
	Data      map[string]interface{} `json:"data,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	TraceID   string                 `json:"trace_id,omitempty"`
}

// QueueStatus represents the current status of the job queue
type QueueStatus struct {
	QueueLength  int       `json:"queue_length"`
	QueueName    string    `json:"queue_name"`
	StatusTime   time.Time `json:"status_time"`
	HealthStatus string    `json:"health_status"`
}

// Client provides Redis-based job queue functionality
type Client struct {
	redis     *redis.Client
	queueName string
	logger    *logger.Logger
}

// NewClient creates a new Redis queue client
func NewClient(redisURL string, log *logger.Logger) (*Client, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis URL is required")
	}

	// Parse Redis URL and create options
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Create Redis client
	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Info("Redis queue client connected successfully",
		"queue_name", "jobs_queue",
		"redis_addr", opts.Addr)

	return &Client{
		redis:     client,
		queueName: "jobs_queue",
		logger:    log,
	}, nil
}

// EnqueueJob adds a job to the queue
func (c *Client) EnqueueJob(ctx context.Context, job *Job) error {
	// Set creation timestamp
	job.CreatedAt = time.Now()

	// Serialize job to JSON
	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	// Add job to Redis list (FIFO queue)
	err = c.redis.LPush(ctx, c.queueName, jobData).Err()
	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	c.logger.Info("Job enqueued successfully",
		"job_id", job.ID,
		"job_type", job.Type,
		"user_id", job.UserID,
		"trace_id", job.TraceID)

	return nil
}

// GetQueueStatus returns the current status of the job queue
func (c *Client) GetQueueStatus(ctx context.Context) (*QueueStatus, error) {
	// Get queue length
	length, err := c.redis.LLen(ctx, c.queueName).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get queue length: %w", err)
	}

	// Test Redis health with a simple ping
	healthStatus := "healthy"
	if err := c.redis.Ping(ctx).Err(); err != nil {
		healthStatus = "unhealthy"
		c.logger.Warn("Redis health check failed", "error", err)
	}

	status := &QueueStatus{
		QueueLength:  int(length),
		QueueName:    c.queueName,
		StatusTime:   time.Now(),
		HealthStatus: healthStatus,
	}

	return status, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	if c.redis != nil {
		return c.redis.Close()
	}
	return nil
}

// IsHealthy checks if the Redis connection is healthy
func (c *Client) IsHealthy(ctx context.Context) bool {
	if c.redis == nil {
		return false
	}
	return c.redis.Ping(ctx).Err() == nil
}