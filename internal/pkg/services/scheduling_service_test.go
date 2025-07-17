package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetUsersInProcessingWindow(ctx context.Context) ([]int, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]int), args.Error(1)
}

func (m *MockUserRepository) GetUserByID(ctx context.Context, userID int) (*database.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.User), args.Error(1)
}

// Mock QueueClient
type MockQueueClient struct {
	mock.Mock
}

func (m *MockQueueClient) EnqueueJob(ctx context.Context, jobType queue.JobType, userID int, data map[string]interface{}) (*queue.Job, error) {
	args := m.Called(ctx, jobType, userID, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*queue.Job), args.Error(1)
}

func (m *MockQueueClient) AcquireUserProcessingLock(ctx context.Context, userID int, duration time.Duration) (bool, error) {
	args := m.Called(ctx, userID, duration)
	return args.Bool(0), args.Error(1)
}

func (m *MockQueueClient) ReleaseUserProcessingLock(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func TestSchedulingService_ProcessScheduledRun(t *testing.T) {
	ctx := context.Background()
	log := logger.New("test")

	tests := []struct {
		name               string
		setupMocks         func(*MockUserRepository, *MockQueueClient)
		expectedJobsCount  int
		expectedError      bool
		errorMessage       string
	}{
		{
			name: "successfully enqueues jobs for all users",
			setupMocks: func(userRepo *MockUserRepository, queueClient *MockQueueClient) {
				// Mock GetUsersInProcessingWindow
				userRepo.On("GetUsersInProcessingWindow", ctx).
					Return([]int{1, 2, 3}, nil)

				// Mock EnqueueJob for each user
				job := &queue.Job{
					ID:     "test-job-id",
					Type:   queue.JobTypeScheduledSync,
					UserID: 1,
				}
				queueClient.On("EnqueueJob", ctx, queue.JobTypeScheduledSync, mock.AnythingOfType("int"), 
					mock.MatchedBy(func(data map[string]interface{}) bool {
						return data["trigger_type"] == "scheduled"
					})).Return(job, nil).Times(3)
			},
			expectedJobsCount: 3,
			expectedError:     false,
		},
		{
			name: "handles no users in processing window",
			setupMocks: func(userRepo *MockUserRepository, queueClient *MockQueueClient) {
				userRepo.On("GetUsersInProcessingWindow", ctx).
					Return([]int{}, nil)
			},
			expectedJobsCount: 0,
			expectedError:     false,
		},
		{
			name: "handles error from GetUsersInProcessingWindow",
			setupMocks: func(userRepo *MockUserRepository, queueClient *MockQueueClient) {
				userRepo.On("GetUsersInProcessingWindow", ctx).
					Return([]int(nil), errors.New("database error"))
			},
			expectedJobsCount: 0,
			expectedError:     true,
			errorMessage:      "failed to get users in processing window",
		},
		{
			name: "continues processing when some jobs fail to enqueue",
			setupMocks: func(userRepo *MockUserRepository, queueClient *MockQueueClient) {
				userRepo.On("GetUsersInProcessingWindow", ctx).
					Return([]int{1, 2, 3}, nil)

				job := &queue.Job{
					ID:     "test-job-id",
					Type:   queue.JobTypeScheduledSync,
					UserID: 1,
				}
				
				// First job succeeds
				queueClient.On("EnqueueJob", ctx, queue.JobTypeScheduledSync, 1, mock.AnythingOfType("map[string]interface {}")).
					Return(job, nil).Once()

				// Second job fails
				queueClient.On("EnqueueJob", ctx, queue.JobTypeScheduledSync, 2, mock.AnythingOfType("map[string]interface {}")).
					Return(nil, errors.New("enqueue error")).Once()

				// Third job succeeds
				job3 := &queue.Job{ID: "test-job-3", Type: queue.JobTypeScheduledSync, UserID: 3}
				queueClient.On("EnqueueJob", ctx, queue.JobTypeScheduledSync, 3, mock.AnythingOfType("map[string]interface {}")).
					Return(job3, nil).Once()
			},
			expectedJobsCount: 2, // Only 2 out of 3 succeed
			expectedError:     false,
		},
		{
			name: "includes scheduled_at timestamp in job data",
			setupMocks: func(userRepo *MockUserRepository, queueClient *MockQueueClient) {
				userRepo.On("GetUsersInProcessingWindow", ctx).
					Return([]int{5}, nil)

				job := &queue.Job{
					ID:     "test-job-5",
					Type:   queue.JobTypeScheduledSync,
					UserID: 5,
				}
				queueClient.On("EnqueueJob", ctx, queue.JobTypeScheduledSync, 5, 
					mock.MatchedBy(func(data map[string]interface{}) bool {
						// Check that scheduled_at is included and is a valid timestamp
						scheduledAt, ok := data["scheduled_at"].(string)
						if !ok {
							return false
						}
						_, err := time.Parse(time.RFC3339, scheduledAt)
						return err == nil
					})).Return(job, nil).Once()
			},
			expectedJobsCount: 1,
			expectedError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			userRepo := new(MockUserRepository)
			queueClient := new(MockQueueClient)

			// Setup mocks
			tt.setupMocks(userRepo, queueClient)

			// Create service
			service := &SchedulingService{
				userRepo:    userRepo,
				queueClient: queueClient,
				logger:      log,
			}

			// Execute
			jobsCount, err := service.ProcessScheduledRun(ctx)

			// Verify results
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedJobsCount, jobsCount)

			// Verify all expectations were met
			userRepo.AssertExpectations(t)
			queueClient.AssertExpectations(t)
		})
	}
}

func TestSchedulingService_JobCreation(t *testing.T) {
	ctx := context.Background()
	log := logger.New("test")

	// Test that jobs are created with correct attributes
	userRepo := new(MockUserRepository)
	queueClient := new(MockQueueClient)

	userID := 42
	userRepo.On("GetUsersInProcessingWindow", ctx).
		Return([]int{userID}, nil)

	// Capture the job data that's enqueued
	var capturedData map[string]interface{}
	returnedJob := &queue.Job{
		ID:     "test-job-42",
		Type:   queue.JobTypeScheduledSync,
		UserID: userID,
		Data:   nil, // Will be set by the actual call
	}
	queueClient.On("EnqueueJob", ctx, queue.JobTypeScheduledSync, userID, 
		mock.MatchedBy(func(data map[string]interface{}) bool {
			capturedData = data
			returnedJob.Data = data
			return true
		})).Return(returnedJob, nil).Once()

	service := &SchedulingService{
		userRepo:    userRepo,
		queueClient: queueClient,
		logger:      log,
	}

	jobsCount, err := service.ProcessScheduledRun(ctx)

	// Verify
	assert.NoError(t, err)
	assert.Equal(t, 1, jobsCount)
	assert.NotNil(t, capturedData)

	// Verify job data attributes
	assert.Equal(t, "scheduled", capturedData["trigger_type"])
	assert.NotEmpty(t, capturedData["scheduled_at"])

	// Verify scheduled_at is a valid timestamp
	scheduledAt, ok := capturedData["scheduled_at"].(string)
	assert.True(t, ok)
	parsedTime, err := time.Parse(time.RFC3339, scheduledAt)
	assert.NoError(t, err)
	assert.WithinDuration(t, time.Now().UTC(), parsedTime, 5*time.Second)
}