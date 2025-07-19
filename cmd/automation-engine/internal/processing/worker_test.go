package processing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/automation"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUserRepository for testing
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetUserByID(ctx context.Context, userID int) (*database.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.User), args.Error(1)
}

func (m *MockUserRepository) GetProcessingConfigForUser(ctx context.Context, userID int) (*database.ProcessingTokens, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.ProcessingTokens), args.Error(1)
}

func (m *MockUserRepository) DecryptToken(encryptedToken []byte) (string, error) {
	args := m.Called(encryptedToken)
	return args.String(0), args.Error(1)
}

// MockTokenPersister for testing
type MockTokenPersister struct {
	mock.Mock
}

func (m *MockTokenPersister) UpdateGoogleTokens(ctx context.Context, userID int, accessToken, refreshToken string, expiry time.Time) error {
	args := m.Called(ctx, userID, accessToken, refreshToken, expiry)
	return args.Error(0)
}

func (m *MockTokenPersister) UpdateStravaTokens(ctx context.Context, userID int, accessToken, refreshToken string, expiry time.Time) error {
	args := m.Called(ctx, userID, accessToken, refreshToken, expiry)
	return args.Error(0)
}

// MockActivityLogRepo for testing
type MockActivityLogRepo struct {
	mock.Mock
}

func (m *MockActivityLogRepo) CreateActivityLog(ctx context.Context, log *database.ActivityLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

// MockQueueClient for testing
type MockQueueClient struct {
	mock.Mock
}

func (m *MockQueueClient) AcquireUserProcessingLock(ctx context.Context, userID int, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, userID, ttl)
	return args.Bool(0), args.Error(1)
}

func (m *MockQueueClient) ReleaseUserProcessingLock(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockQueueClient) EnqueueJob(ctx context.Context, jobType queue.JobType, userID int, data map[string]interface{}) (*queue.Job, error) {
	args := m.Called(ctx, jobType, userID, data)
	if job := args.Get(0); job != nil {
		return job.(*queue.Job), args.Error(1)
	}
	return nil, args.Error(1)
}

// Test ProcessUserWithData with configuration error
func TestProcessUserWithData_ConfigError(t *testing.T) {
	// Create mocks
	mockUserRepo := new(MockUserRepository)
	mockTokenPersister := new(MockTokenPersister)
	mockActivityLogRepo := new(MockActivityLogRepo)
	mockQueueClient := new(MockQueueClient)

	// Setup mocks
	mockQueueClient.On("AcquireUserProcessingLock", mock.Anything, 123, 10*time.Minute).Return(true, nil)
	mockQueueClient.On("ReleaseUserProcessingLock", mock.Anything, 123).Return(nil)
	mockUserRepo.On("GetUserByID", mock.Anything, 123).Return(nil, errors.New("user not found"))
	mockActivityLogRepo.On("CreateActivityLog", mock.Anything, mock.Anything).Return(nil)

	// Create config service with mock
	configService := automation.NewConfigService(mockUserRepo, logger.New("test"))

	// Create worker
	worker := NewWorker(
		configService,
		mockTokenPersister,
		mockActivityLogRepo,
		mockQueueClient, // jobsQueueClient
		nil, // notification queue client
		"strava-client-id",
		"strava-client-secret",
		"google-client-id",
		"google-client-secret",
		"google-redirect-url",
		logger.New("test"),
	)

	// Process user
	result := worker.ProcessUserWithData(context.Background(), 123, string(queue.JobTypeManualSync), nil)

	// Verify
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "CONFIG_ERROR", result.ErrorType)
	assert.Contains(t, result.Error, "user not found")

	// Verify mocks
	mockUserRepo.AssertExpectations(t)
	mockQueueClient.AssertExpectations(t)
	mockActivityLogRepo.AssertExpectations(t)
}

// Test ProcessUserWithData with automation disabled
func TestProcessUserWithData_AutomationDisabled(t *testing.T) {
	// Create mocks
	mockUserRepo := new(MockUserRepository)
	mockTokenPersister := new(MockTokenPersister)
	mockActivityLogRepo := new(MockActivityLogRepo)
	mockQueueClient := new(MockQueueClient)

	// Create test user with automation disabled
	spreadsheetID := "test-spreadsheet-id"
	stravaAthleteID := int64(12345)
	testUser := &database.User{
		ID:                123,
		Email:             "test@example.com",
		AutomationEnabled: false,
		Timezone:          "UTC",
		SpreadsheetID:     &spreadsheetID,
		StravaAthleteID:   &stravaAthleteID,
	}

	// Create processing tokens
	tokenExpiry := time.Now().Add(time.Hour)
	processingTokens := &database.ProcessingTokens{
		StravaRefreshToken:  "strava-refresh",
		StravaAccessToken:   "strava-access",
		StravaTokenExpiry:   &tokenExpiry,
		GoogleRefreshToken:  "google-refresh",
		GoogleAccessToken:   "google-access",
		GoogleTokenExpiry:   &tokenExpiry,
		SpreadsheetID:       &spreadsheetID,
		Timezone:            "UTC",
		Email:               "test@example.com",
		StravaAthleteID:     &stravaAthleteID,
	}

	// Setup mocks
	mockQueueClient.On("AcquireUserProcessingLock", mock.Anything, 123, 10*time.Minute).Return(true, nil)
	mockQueueClient.On("ReleaseUserProcessingLock", mock.Anything, 123).Return(nil)
	mockUserRepo.On("GetUserByID", mock.Anything, 123).Return(testUser, nil)
	mockUserRepo.On("GetProcessingConfigForUser", mock.Anything, 123).Return(processingTokens, nil)
	mockActivityLogRepo.On("CreateActivityLog", mock.Anything, mock.Anything).Return(nil)

	// Create config service with mock
	configService := automation.NewConfigService(mockUserRepo, logger.New("test"))

	// Create worker
	worker := NewWorker(
		configService,
		mockTokenPersister,
		mockActivityLogRepo,
		mockQueueClient, // jobsQueueClient
		nil, // notification queue client
		"strava-client-id",
		"strava-client-secret",
		"google-client-id",
		"google-client-secret",
		"google-redirect-url",
		logger.New("test"),
	)

	// Process user
	result := worker.ProcessUserWithData(context.Background(), 123, string(queue.JobTypeManualSync), nil)

	// Verify
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "AUTOMATION_DISABLED", result.ErrorType)
	assert.Contains(t, result.Error, "Automation is disabled")

	// Verify mocks
	mockUserRepo.AssertExpectations(t)
	mockQueueClient.AssertExpectations(t)
	mockActivityLogRepo.AssertExpectations(t)
}

// Test ProcessUserWithData with lock acquisition failure
func TestProcessUserWithData_LockAcquisitionFailure(t *testing.T) {
	// Create mocks
	mockUserRepo := new(MockUserRepository)
	mockTokenPersister := new(MockTokenPersister)
	mockActivityLogRepo := new(MockActivityLogRepo)
	mockQueueClient := new(MockQueueClient)

	// Setup mocks - lock acquisition fails
	mockQueueClient.On("AcquireUserProcessingLock", mock.Anything, 123, 10*time.Minute).Return(false, errors.New("redis error"))

	// Create config service with mock
	configService := automation.NewConfigService(mockUserRepo, logger.New("test"))

	// Create worker
	worker := NewWorker(
		configService,
		mockTokenPersister,
		mockActivityLogRepo,
		mockQueueClient, // jobsQueueClient
		nil, // notification queue client
		"strava-client-id",
		"strava-client-secret",
		"google-client-id",
		"google-client-secret",
		"google-redirect-url",
		logger.New("test"),
	)

	// Process user
	result := worker.ProcessUserWithData(context.Background(), 123, string(queue.JobTypeManualSync), nil)

	// Verify
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "LOCK_ERROR", result.ErrorType)
	assert.Contains(t, result.Error, "Failed to acquire processing lock")

	// Verify mocks
	mockQueueClient.AssertExpectations(t)
}

// Test ProcessUserWithData when user is already being processed
func TestProcessUserWithData_AlreadyProcessing(t *testing.T) {
	// Create mocks
	mockUserRepo := new(MockUserRepository)
	mockTokenPersister := new(MockTokenPersister)
	mockActivityLogRepo := new(MockActivityLogRepo)
	mockQueueClient := new(MockQueueClient)

	// Setup mocks - lock not acquired (already locked)
	mockQueueClient.On("AcquireUserProcessingLock", mock.Anything, 123, 10*time.Minute).Return(false, nil)

	// Create config service with mock
	configService := automation.NewConfigService(mockUserRepo, logger.New("test"))

	// Create worker
	worker := NewWorker(
		configService,
		mockTokenPersister,
		mockActivityLogRepo,
		mockQueueClient, // jobsQueueClient
		nil, // notification queue client
		"strava-client-id",
		"strava-client-secret",
		"google-client-id",
		"google-client-secret",
		"google-redirect-url",
		logger.New("test"),
	)

	// Process user
	result := worker.ProcessUserWithData(context.Background(), 123, string(queue.JobTypeManualSync), nil)

	// Verify
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "ALREADY_PROCESSING", result.ErrorType)
	assert.Contains(t, result.Error, "User is already being processed")

	// Verify mocks
	mockQueueClient.AssertExpectations(t)
}

// Test ProcessUserWithData with scheduled trigger
func TestProcessUserWithData_ScheduledTrigger(t *testing.T) {
	// Create mocks
	mockUserRepo := new(MockUserRepository)
	mockTokenPersister := new(MockTokenPersister)
	mockActivityLogRepo := new(MockActivityLogRepo)
	mockQueueClient := new(MockQueueClient)

	// Create valid config
	spreadsheetID := "test-spreadsheet-id"
	tokenExpiry := time.Now().Add(time.Hour)
	stravaAthleteID := int64(12345)
	testUser := &database.User{
		ID:                123,
		Email:             "test@example.com",
		AutomationEnabled: true,
		Timezone:          "UTC",
		SpreadsheetID:     &spreadsheetID,
		StravaAthleteID:   &stravaAthleteID,
	}

	// Create processing tokens
	processingTokens := &database.ProcessingTokens{
		StravaRefreshToken:  "strava-refresh",
		StravaAccessToken:   "strava-access",
		StravaTokenExpiry:   &tokenExpiry,
		GoogleRefreshToken:  "google-refresh",
		GoogleAccessToken:   "google-access",
		GoogleTokenExpiry:   &tokenExpiry,
		SpreadsheetID:       &spreadsheetID,
		Timezone:            "UTC",
		Email:               "test@example.com",
		StravaAthleteID:     &stravaAthleteID,
	}

	// Setup mocks
	mockQueueClient.On("AcquireUserProcessingLock", mock.Anything, 123, 10*time.Minute).Return(true, nil)
	mockQueueClient.On("ReleaseUserProcessingLock", mock.Anything, 123).Return(nil)
	mockUserRepo.On("GetUserByID", mock.Anything, 123).Return(testUser, nil)
	mockUserRepo.On("GetProcessingConfigForUser", mock.Anything, 123).Return(processingTokens, nil)
	
	// Since we can't easily mock the entire processing flow, we'll expect the activity log
	// to show that processing was attempted
	mockActivityLogRepo.On("CreateActivityLog", mock.Anything, mock.MatchedBy(func(log *database.ActivityLog) bool {
		return log.UserID == 123 && log.ProcessingType == "daily"
	})).Return(nil)

	// Create config service with mock
	configService := automation.NewConfigService(mockUserRepo, logger.New("test"))

	// Create worker without queue client to simplify test
	worker := NewWorker(
		configService,
		mockTokenPersister,
		mockActivityLogRepo,
		mockQueueClient,
		nil, // notification queue client
		"strava-client-id",
		"strava-client-secret",
		"google-client-id",
		"google-client-secret",
		"google-redirect-url",
		logger.New("test"),
	)

	// Create job data with scheduled trigger
	jobData := map[string]interface{}{
		"trigger_type": "scheduled",
	}

	// Process user - will fail due to missing sheets client but that's ok for this test
	result := worker.ProcessUserWithData(context.Background(), 123, string(queue.JobTypeScheduledSync), jobData)

	// Verify that it attempted to process
	assert.NotNil(t, result)
	assert.Equal(t, 123, result.UserID)

	// Verify mocks
	mockUserRepo.AssertExpectations(t)
	mockQueueClient.AssertExpectations(t)
}

// Test ProcessUserWithData without queue client (nil)
func TestProcessUserWithData_NoQueueClient(t *testing.T) {
	// Create mocks
	mockUserRepo := new(MockUserRepository)
	mockTokenPersister := new(MockTokenPersister)
	mockActivityLogRepo := new(MockActivityLogRepo)

	// Create test user with automation disabled to exit early
	spreadsheetID := "test-spreadsheet-id"
	stravaAthleteID := int64(12345)
	testUser := &database.User{
		ID:                123,
		Email:             "test@example.com",
		AutomationEnabled: false,
		Timezone:          "UTC",
		SpreadsheetID:     &spreadsheetID,
		StravaAthleteID:   &stravaAthleteID,
	}

	// Create processing tokens
	tokenExpiry := time.Now().Add(time.Hour)
	processingTokens := &database.ProcessingTokens{
		StravaRefreshToken:  "strava-refresh",
		StravaAccessToken:   "strava-access",
		StravaTokenExpiry:   &tokenExpiry,
		GoogleRefreshToken:  "google-refresh",
		GoogleAccessToken:   "google-access",
		GoogleTokenExpiry:   &tokenExpiry,
		SpreadsheetID:       &spreadsheetID,
		Timezone:            "UTC",
		Email:               "test@example.com",
		StravaAthleteID:     &stravaAthleteID,
	}

	// Setup mocks
	mockUserRepo.On("GetUserByID", mock.Anything, 123).Return(testUser, nil)
	mockUserRepo.On("GetProcessingConfigForUser", mock.Anything, 123).Return(processingTokens, nil)
	mockActivityLogRepo.On("CreateActivityLog", mock.Anything, mock.Anything).Return(nil)

	// Create config service with mock
	configService := automation.NewConfigService(mockUserRepo, logger.New("test"))

	// Create worker without queue client
	worker := NewWorker(
		configService,
		mockTokenPersister,
		mockActivityLogRepo,
		nil, // No jobs queue client
		nil, // notification queue client
		"strava-client-id",
		"strava-client-secret",
		"google-client-id",
		"google-client-secret",
		"google-redirect-url",
		logger.New("test"),
	)

	// Process user - should work without locking
	result := worker.ProcessUserWithData(context.Background(), 123, string(queue.JobTypeManualSync), nil)

	// Verify
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "AUTOMATION_DISABLED", result.ErrorType)

	// Verify mocks
	mockUserRepo.AssertExpectations(t)
	mockActivityLogRepo.AssertExpectations(t)
}