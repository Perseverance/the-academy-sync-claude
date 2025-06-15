package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
)

// MockConfigService implements automation.ConfigService interface for testing
type MockConfigService struct {
	validateUserResult error
}

func (m *MockConfigService) ValidateUserCanBeProcessed(ctx context.Context, userID int) error {
	return m.validateUserResult
}

// MockQueueClient implements queue operations for testing
type MockQueueClient struct {
	enqueueError  error
	enqueueResult *queue.Job
	queueLength   int64
	healthError   error
}

func (m *MockQueueClient) EnqueueJob(ctx context.Context, userID int, triggerType string) (*queue.Job, error) {
	if m.enqueueError != nil {
		return nil, m.enqueueError
	}
	
	if m.enqueueResult != nil {
		return m.enqueueResult, nil
	}
	
	// Default successful response
	return &queue.Job{
		UserID:         userID,
		TraceID:        "test-trace-id-123",
		TriggerType:    triggerType,
		CreatedAt:      time.Now(),
		TimeoutSeconds: 300,
	}, nil
}

func (m *MockQueueClient) GetQueueLength(ctx context.Context) (int64, error) {
	return m.queueLength, nil
}

func (m *MockQueueClient) HealthCheck(ctx context.Context) error {
	return m.healthError
}

func TestSyncService_TriggerManualSync(t *testing.T) {
	log := logger.New("test")

	t.Run("SuccessfulSync", func(t *testing.T) {
		mockConfig := &MockConfigService{validateUserResult: nil}
		mockQueue := &MockQueueClient{}
		
		syncService := NewSyncService(mockConfig, mockQueue, log)
		
		response, err := syncService.TriggerManualSync(context.Background(), 123)
		
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		
		if response == nil {
			t.Fatal("Expected response, got nil")
		}
		
		if !response.Success {
			t.Error("Expected success to be true")
		}
		
		if response.TraceID != "test-trace-id-123" {
			t.Errorf("Expected trace ID 'test-trace-id-123', got %s", response.TraceID)
		}
		
		if response.EstimatedCompletionSeconds != 60 {
			t.Errorf("Expected estimated completion 60 seconds, got %d", response.EstimatedCompletionSeconds)
		}
	})

	t.Run("UserNotFound", func(t *testing.T) {
		mockConfig := &MockConfigService{
			validateUserResult: fmt.Errorf("user not found: 999"),
		}
		mockQueue := &MockQueueClient{}
		
		syncService := NewSyncService(mockConfig, mockQueue, log)
		
		response, err := syncService.TriggerManualSync(context.Background(), 999)
		
		if response != nil {
			t.Error("Expected nil response for user not found")
		}
		
		if err == nil {
			t.Fatal("Expected error for user not found")
		}
		
		syncErr, ok := err.(*SyncError)
		if !ok {
			t.Fatalf("Expected SyncError, got %T", err)
		}
		
		if syncErr.Type != SyncErrorUserNotFound {
			t.Errorf("Expected error type %s, got %s", SyncErrorUserNotFound, syncErr.Type)
		}
	})

	t.Run("UserNotConfigured", func(t *testing.T) {
		mockConfig := &MockConfigService{
			validateUserResult: fmt.Errorf("user missing essential configuration: [strava_refresh_token]"),
		}
		mockQueue := &MockQueueClient{}
		
		syncService := NewSyncService(mockConfig, mockQueue, log)
		
		response, err := syncService.TriggerManualSync(context.Background(), 123)
		
		if response != nil {
			t.Error("Expected nil response for unconfigured user")
		}
		
		if err == nil {
			t.Fatal("Expected error for unconfigured user")
		}
		
		syncErr, ok := err.(*SyncError)
		if !ok {
			t.Fatalf("Expected SyncError, got %T", err)
		}
		
		if syncErr.Type != SyncErrorUserNotConfigured {
			t.Errorf("Expected error type %s, got %s", SyncErrorUserNotConfigured, syncErr.Type)
		}
	})

	t.Run("QueueUnavailable", func(t *testing.T) {
		mockConfig := &MockConfigService{validateUserResult: nil}
		mockQueue := &MockQueueClient{
			enqueueError: fmt.Errorf("Redis connection failed"),
		}
		
		syncService := NewSyncService(mockConfig, mockQueue, log)
		
		response, err := syncService.TriggerManualSync(context.Background(), 123)
		
		if response != nil {
			t.Error("Expected nil response for queue unavailable")
		}
		
		if err == nil {
			t.Fatal("Expected error for queue unavailable")
		}
		
		syncErr, ok := err.(*SyncError)
		if !ok {
			t.Fatalf("Expected SyncError, got %T", err)
		}
		
		if syncErr.Type != SyncErrorQueueUnavailable {
			t.Errorf("Expected error type %s, got %s", SyncErrorQueueUnavailable, syncErr.Type)
		}
	})
}

func TestSyncService_GetQueueStatus(t *testing.T) {
	log := logger.New("test")

	t.Run("HealthyQueue", func(t *testing.T) {
		mockConfig := &MockConfigService{}
		mockQueue := &MockQueueClient{
			queueLength: 5,
			healthError: nil,
		}
		
		syncService := NewSyncService(mockConfig, mockQueue, log)
		
		status, err := syncService.GetQueueStatus(context.Background())
		
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		
		if status == nil {
			t.Fatal("Expected status, got nil")
		}
		
		if status["queue_length"] != int64(5) {
			t.Errorf("Expected queue length 5, got %v", status["queue_length"])
		}
		
		if status["health_status"] != "healthy" {
			t.Errorf("Expected health status 'healthy', got %v", status["health_status"])
		}
		
		if status["queue_name"] != queue.JobsQueueName {
			t.Errorf("Expected queue name %s, got %v", queue.JobsQueueName, status["queue_name"])
		}
	})

	t.Run("UnhealthyQueue", func(t *testing.T) {
		mockConfig := &MockConfigService{}
		mockQueue := &MockQueueClient{
			queueLength: 0,
			healthError: fmt.Errorf("Redis connection failed"),
		}
		
		syncService := NewSyncService(mockConfig, mockQueue, log)
		
		status, err := syncService.GetQueueStatus(context.Background())
		
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		
		if status == nil {
			t.Fatal("Expected status, got nil")
		}
		
		if status["health_status"] != "unhealthy" {
			t.Errorf("Expected health status 'unhealthy', got %v", status["health_status"])
		}
		
		if status["health_error"] == nil {
			t.Error("Expected health error to be set")
		}
	})
}