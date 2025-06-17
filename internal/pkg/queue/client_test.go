package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *Client) {
	// Create a mock Redis server
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Create queue client
	log := logger.New("test")
	client, err := NewClient("redis://"+mr.Addr(), "test_queue", log)
	require.NoError(t, err)

	return mr, client
}

func TestNewClient(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()
		defer client.Close()

		assert.NotNil(t, client)
		assert.Equal(t, "test_queue", client.queueName)
	})

	t.Run("invalid Redis URL", func(t *testing.T) {
		log := logger.New("test")
		client, err := NewClient("invalid://url", "test_queue", log)

		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "failed to parse Redis URL")
	})

	t.Run("connection failure", func(t *testing.T) {
		log := logger.New("test")
		client, err := NewClient("redis://localhost:9999", "test_queue", log)

		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "failed to connect to Redis")
	})
}

func TestEnqueueJob(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	t.Run("successful enqueue", func(t *testing.T) {
		data := map[string]interface{}{
			"test_key": "test_value",
		}

		job, err := client.EnqueueJob(ctx, JobTypeManualSync, 123, data)

		require.NoError(t, err)
		assert.NotNil(t, job)
		assert.NotEmpty(t, job.ID)
		assert.NotEmpty(t, job.TraceID)
		assert.Equal(t, JobTypeManualSync, job.Type)
		assert.Equal(t, 123, job.UserID)
		assert.Equal(t, data, job.Data)
		assert.WithinDuration(t, time.Now(), job.CreatedAt, 2*time.Second)

		// Verify job was added to queue
		length, err := client.GetQueueLength(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1), length)
	})

	t.Run("enqueue multiple jobs", func(t *testing.T) {
		mr.FlushAll()

		for i := 0; i < 3; i++ {
			job, err := client.EnqueueJob(ctx, JobTypeScheduledSync, i+1, nil)
			require.NoError(t, err)
			assert.NotNil(t, job)
		}

		length, err := client.GetQueueLength(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(3), length)
	})
}

func TestDequeueJob(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	t.Run("successful dequeue", func(t *testing.T) {
		// First enqueue a job
		originalJob, err := client.EnqueueJob(ctx, JobTypeManualSync, 456, map[string]interface{}{"key": "value"})
		require.NoError(t, err)

		// Then dequeue it
		dequeuedJob, err := client.DequeueJob(ctx)

		require.NoError(t, err)
		assert.NotNil(t, dequeuedJob)
		assert.Equal(t, originalJob.ID, dequeuedJob.ID)
		assert.Equal(t, originalJob.TraceID, dequeuedJob.TraceID)
		assert.Equal(t, originalJob.Type, dequeuedJob.Type)
		assert.Equal(t, originalJob.UserID, dequeuedJob.UserID)
		assert.Equal(t, originalJob.Data, dequeuedJob.Data)
	})

	t.Run("dequeue with empty queue and timeout", func(t *testing.T) {
		mr.FlushAll()

		// Use a context with timeout for this test
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		job, err := client.DequeueJob(ctx)

		// Should timeout
		assert.Error(t, err)
		assert.Nil(t, job)
	})

	t.Run("FIFO order", func(t *testing.T) {
		mr.FlushAll()

		// Enqueue multiple jobs
		job1, _ := client.EnqueueJob(ctx, JobTypeManualSync, 1, nil)
		job2, _ := client.EnqueueJob(ctx, JobTypeScheduledSync, 2, nil)
		job3, _ := client.EnqueueJob(ctx, JobTypeManualSync, 3, nil)

		// Dequeue and verify FIFO order
		dequeued1, err := client.DequeueJob(ctx)
		require.NoError(t, err)
		assert.Equal(t, job1.ID, dequeued1.ID)

		dequeued2, err := client.DequeueJob(ctx)
		require.NoError(t, err)
		assert.Equal(t, job2.ID, dequeued2.ID)

		dequeued3, err := client.DequeueJob(ctx)
		require.NoError(t, err)
		assert.Equal(t, job3.ID, dequeued3.ID)
	})
}

func TestGetQueueLength(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	t.Run("empty queue", func(t *testing.T) {
		mr.FlushAll()

		length, err := client.GetQueueLength(ctx)

		require.NoError(t, err)
		assert.Equal(t, int64(0), length)
	})

	t.Run("queue with items", func(t *testing.T) {
		mr.FlushAll()

		// Add some jobs
		for i := 0; i < 5; i++ {
			_, err := client.EnqueueJob(ctx, JobTypeManualSync, i, nil)
			require.NoError(t, err)
		}

		length, err := client.GetQueueLength(ctx)

		require.NoError(t, err)
		assert.Equal(t, int64(5), length)
	})
}

func TestHealthCheck(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer client.Close()

	ctx := context.Background()

	t.Run("healthy connection", func(t *testing.T) {
		err := client.HealthCheck(ctx)
		assert.NoError(t, err)
	})

	t.Run("unhealthy connection", func(t *testing.T) {
		mr.Close() // Close the mock Redis server

		err := client.HealthCheck(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis health check failed")
	})
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "redis nil error (key not found)",
			err:      redis.Nil,
			expected: false,
		},
		{
			name:     "connection refused",
			err:      fmt.Errorf("dial tcp: connect: connection refused"),
			expected: true,
		},
		{
			name:     "no such host",
			err:      fmt.Errorf("dial tcp: lookup redis: no such host"),
			expected: true,
		},
		{
			name:     "i/o timeout",
			err:      fmt.Errorf("i/o timeout"),
			expected: true,
		},
		{
			name:     "regular error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsConnectionError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJobSerialization(t *testing.T) {
	job := &Job{
		ID:     "test-id",
		Type:   JobTypeManualSync,
		UserID: 789,
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": float64(123), // JSON unmarshaling converts numbers to float64
		},
		CreatedAt: time.Now().UTC(),
		TraceID:   "trace-123",
	}

	// Marshal
	data, err := json.Marshal(job)
	require.NoError(t, err)

	// Unmarshal
	var decoded Job
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, job.ID, decoded.ID)
	assert.Equal(t, job.Type, decoded.Type)
	assert.Equal(t, job.UserID, decoded.UserID)
	assert.Equal(t, job.Data, decoded.Data)
	assert.Equal(t, job.TraceID, decoded.TraceID)
	assert.WithinDuration(t, job.CreatedAt, decoded.CreatedAt, time.Millisecond)
}
