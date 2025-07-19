package internal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockQueueClient for testing
type MockQueueClient struct {
	mock.Mock
}

func (m *MockQueueClient) DequeueJob(ctx context.Context) (*queue.Job, error) {
	args := m.Called(ctx)
	if job := args.Get(0); job != nil {
		return job.(*queue.Job), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockQueueClient) EnqueueJob(ctx context.Context, jobType queue.JobType, userID int, data map[string]interface{}) (*queue.Job, error) {
	args := m.Called(ctx, jobType, userID, data)
	if job := args.Get(0); job != nil {
		return job.(*queue.Job), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockQueueClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockQueueClient) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockQueueClient) GetQueueLength(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQueueClient) AcquireUserProcessingLock(ctx context.Context, userID int, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, userID, ttl)
	return args.Bool(0), args.Error(1)
}

func (m *MockQueueClient) ReleaseUserProcessingLock(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockQueueClient) IsUserProcessingLocked(ctx context.Context, userID int) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

// MockNotificationService for testing
type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) ProcessNotification(ctx context.Context, notification *NotificationJob) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

// Test processJob with valid notification data
func TestProcessJob_Success(t *testing.T) {
	mockQueue := new(MockQueueClient)
	mockService := new(MockNotificationService)
	logger := logger.New("test")

	// Create worker with mock queue client
	worker := &Worker{
		queueClient: mockQueue,
		service:     mockService,
		logger:      logger,
	}

	job := &queue.Job{
		ID:     "test-job-1",
		Type:   "notification",
		UserID: 123,
		Data: map[string]interface{}{
			"user_id":    123,
			"user_email": "test@example.com",
			"user_name":  "Test User",
			"run_date":   time.Now().Format(time.RFC3339),
			"logs": []map[string]interface{}{
				{
					"date":             "2024-01-15",
					"status":           "success",
					"summary_message":  "1 activity logged",
					"activities_found": 1,
				},
			},
		},
		CreatedAt: time.Now(),
	}

	// Expect service to process notification
	mockService.On("ProcessNotification", mock.Anything, mock.MatchedBy(func(n *NotificationJob) bool {
		return n.UserID == 123 && n.UserEmail == "test@example.com"
	})).Return(nil)

	// No acknowledgment needed - job is removed by BRPOP

	// Process the job
	worker.processJob(context.Background(), job)

	// Verify expectations
	mockService.AssertExpectations(t)
	mockQueue.AssertExpectations(t)
}

// Test processJob with invalid data
func TestProcessJob_InvalidData(t *testing.T) {
	mockQueue := new(MockQueueClient)
	mockService := new(MockNotificationService)
	logger := logger.New("test")

	worker := NewWorker(&queue.Client{}, mockService, logger)
	worker.queueClient = mockQueue

	// Job with missing email
	job := &queue.Job{
		ID:     "test-job-2",
		Type:   "notification",
		UserID: 123,
		Data: map[string]interface{}{
			"user_id": 123,
			// missing user_email
		},
		CreatedAt: time.Now(),
	}

	// No acknowledgment needed - job is removed by BRPOP

	// Process the job
	worker.processJob(context.Background(), job)

	// Service should not be called
	mockService.AssertNotCalled(t, "ProcessNotification")
	mockQueue.AssertExpectations(t)
}

// Test processJob with service error
func TestProcessJob_ServiceError(t *testing.T) {
	mockQueue := new(MockQueueClient)
	mockService := new(MockNotificationService)
	logger := logger.New("test")

	worker := NewWorker(&queue.Client{}, mockService, logger)
	worker.queueClient = mockQueue

	job := &queue.Job{
		ID:     "test-job-3",
		Type:   "notification",
		UserID: 123,
		Data: map[string]interface{}{
			"user_id":    123,
			"user_email": "test@example.com",
			"user_name":  "Test User",
			"run_date":   time.Now().Format(time.RFC3339),
			"logs":       []map[string]interface{}{},
		},
		CreatedAt: time.Now(),
	}

	// Service returns error
	mockService.On("ProcessNotification", mock.Anything, mock.Anything).
		Return(errors.New("email send failed"))

	// No acknowledgment needed - job is removed by BRPOP

	// Process the job
	worker.processJob(context.Background(), job)

	// Verify expectations
	mockService.AssertExpectations(t)
	mockQueue.AssertExpectations(t)
}

// Test decodeJobData
func TestDecodeJobData(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		data := map[string]interface{}{
			"user_id":    123,
			"user_email": "test@example.com",
			"user_name":  "Test User",
			"run_date":   "2024-01-15T10:00:00Z",
			"logs": []map[string]interface{}{
				{
					"date":             "2024-01-15",
					"status":           "success",
					"summary_message":  "Test message",
					"activities_found": 1,
				},
			},
		}

		var notification NotificationJob
		err := decodeJobData(data, &notification)

		assert.NoError(t, err)
		assert.Equal(t, 123, notification.UserID)
		assert.Equal(t, "test@example.com", notification.UserEmail)
		assert.Equal(t, "Test User", notification.UserName)
		assert.Len(t, notification.Logs, 1)
		assert.Equal(t, "success", notification.Logs[0].Status)
	})

	t.Run("invalid data type", func(t *testing.T) {
		data := map[string]interface{}{
			"user_id": "not-a-number", // Should be int
			"user_email": "test@example.com",
		}

		var notification NotificationJob
		err := decodeJobData(data, &notification)

		assert.Error(t, err)
	})
}

// Test ProcessJobs context cancellation
func TestProcessJobs_ContextCancellation(t *testing.T) {
	mockQueue := new(MockQueueClient)
	mockService := new(MockNotificationService)
	logger := logger.New("test")

	worker := NewWorker(&queue.Client{}, mockService, logger)
	worker.queueClient = mockQueue

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Mock dequeue to return no jobs initially
	mockQueue.On("DequeueJob", mock.Anything).
		Return(nil, context.DeadlineExceeded).Maybe()

	// Start processing in goroutine
	done := make(chan bool)
	go func() {
		worker.ProcessJobs(ctx)
		done <- true
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for completion
	select {
	case <-done:
		// Success - worker stopped
	case <-time.After(1 * time.Second):
		t.Error("Worker did not stop after context cancellation")
	}
}