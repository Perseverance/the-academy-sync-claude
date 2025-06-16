package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
)

type mockUserRepository struct {
	users map[int]*database.User
	err   error
}

func (m *mockUserRepository) GetUserByID(ctx context.Context, userID int) (*database.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.users[userID], nil
}

func (m *mockUserRepository) GetUserByGoogleID(ctx context.Context, googleID string) (*database.User, error) {
	return nil, nil
}

func (m *mockUserRepository) GetUserByEmail(ctx context.Context, email string) (*database.User, error) {
	return nil, nil
}

func (m *mockUserRepository) CreateUser(ctx context.Context, req *database.CreateUserRequest) (*database.User, error) {
	return nil, nil
}

func (m *mockUserRepository) UpdateUserTokens(ctx context.Context, req *database.UpdateUserTokensRequest) error {
	return nil
}

func (m *mockUserRepository) UpdateStravaConnection(ctx context.Context, userID int, athleteID int64, athleteName, profilePictureURL, accessToken, refreshToken string, expiry *time.Time) error {
	return nil
}

func (m *mockUserRepository) DisconnectStrava(ctx context.Context, userID int) error {
	return nil
}

func (m *mockUserRepository) UpdateSpreadsheetID(ctx context.Context, userID int, spreadsheetID *string) error {
	return nil
}

func setupSyncServiceTest(t *testing.T) (*miniredis.Miniredis, *queue.Client, *SyncService, *mockUserRepository) {
	// Setup Redis
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Setup queue client
	log := logger.New("test")
	queueClient, err := queue.NewClient("redis://"+mr.Addr(), "test_queue", log)
	require.NoError(t, err)

	// Setup mock user repository
	mockRepo := &mockUserRepository{
		users: make(map[int]*database.User),
	}

	// Create sync service
	syncService := NewSyncService(mockRepo, queueClient, log)

	return mr, queueClient, syncService, mockRepo
}

func TestTriggerManualSync(t *testing.T) {
	ctx := context.Background()

	t.Run("successful sync trigger", func(t *testing.T) {
		mr, queueClient, syncService, mockRepo := setupSyncServiceTest(t)
		defer mr.Close()
		defer queueClient.Close()

		// Setup a fully configured user
		spreadsheetID := "test-spreadsheet-123"
		mockRepo.users[1] = &database.User{
			ID:                 1,
			Email:              "test@example.com",
			StravaRefreshToken: []byte("encrypted-strava-token"),
			SpreadsheetID:      &spreadsheetID,
			AutomationEnabled:  true,
		}

		// Trigger sync
		err := syncService.TriggerManualSync(ctx, 1)

		assert.NoError(t, err)

		// Verify job was enqueued
		length, err := queueClient.GetQueueLength(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1), length)

		// Verify job content
		job, err := queueClient.DequeueJob(ctx)
		require.NoError(t, err)
		assert.Equal(t, queue.JobTypeManualSync, job.Type)
		assert.Equal(t, 1, job.UserID)
		assert.Equal(t, "manual", job.Data["trigger_type"])
		assert.Equal(t, "test@example.com", job.Data["email"])
		assert.Equal(t, "test-spreadsheet-123", job.Data["spreadsheet_id"])
	})

	t.Run("user not found", func(t *testing.T) {
		mr, queueClient, syncService, _ := setupSyncServiceTest(t)
		defer mr.Close()
		defer queueClient.Close()

		// Trigger sync for non-existent user
		err := syncService.TriggerManualSync(ctx, 999)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")

		// Verify no job was enqueued
		length, err := queueClient.GetQueueLength(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), length)
	})

	t.Run("no Strava connection", func(t *testing.T) {
		mr, queueClient, syncService, mockRepo := setupSyncServiceTest(t)
		defer mr.Close()
		defer queueClient.Close()

		// Setup user without Strava connection
		spreadsheetID := "test-spreadsheet-123"
		mockRepo.users[2] = &database.User{
			ID:                 2,
			Email:              "test2@example.com",
			StravaRefreshToken: nil, // No Strava token
			SpreadsheetID:      &spreadsheetID,
		}

		// Trigger sync
		err := syncService.TriggerManualSync(ctx, 2)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "strava connection required")

		// Verify no job was enqueued
		length, err := queueClient.GetQueueLength(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), length)
	})

	t.Run("no spreadsheet configured", func(t *testing.T) {
		mr, queueClient, syncService, mockRepo := setupSyncServiceTest(t)
		defer mr.Close()
		defer queueClient.Close()

		// Setup user without spreadsheet
		mockRepo.users[3] = &database.User{
			ID:                 3,
			Email:              "test3@example.com",
			StravaRefreshToken: []byte("encrypted-strava-token"),
			SpreadsheetID:      nil, // No spreadsheet
		}

		// Trigger sync
		err := syncService.TriggerManualSync(ctx, 3)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "spreadsheet configuration required")

		// Verify no job was enqueued
		length, err := queueClient.GetQueueLength(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), length)
	})

	t.Run("empty spreadsheet ID", func(t *testing.T) {
		mr, queueClient, syncService, mockRepo := setupSyncServiceTest(t)
		defer mr.Close()
		defer queueClient.Close()

		// Setup user with empty spreadsheet ID
		emptyID := ""
		mockRepo.users[4] = &database.User{
			ID:                 4,
			Email:              "test4@example.com",
			StravaRefreshToken: []byte("encrypted-strava-token"),
			SpreadsheetID:      &emptyID, // Empty string
		}

		// Trigger sync
		err := syncService.TriggerManualSync(ctx, 4)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "spreadsheet configuration required")
	})

	t.Run("database error", func(t *testing.T) {
		mr, queueClient, syncService, mockRepo := setupSyncServiceTest(t)
		defer mr.Close()
		defer queueClient.Close()

		// Setup repository to return error
		mockRepo.err = sql.ErrConnDone

		// Trigger sync
		err := syncService.TriggerManualSync(ctx, 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch user")
	})
}

func TestGetUserSyncStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("eligible user", func(t *testing.T) {
		mr, queueClient, syncService, mockRepo := setupSyncServiceTest(t)
		defer mr.Close()
		defer queueClient.Close()

		// Setup fully configured user
		spreadsheetID := "test-spreadsheet-123"
		mockRepo.users[1] = &database.User{
			ID:                 1,
			Email:              "test@example.com",
			StravaRefreshToken: []byte("encrypted-strava-token"),
			SpreadsheetID:      &spreadsheetID,
		}

		eligible, reason, err := syncService.GetUserSyncStatus(ctx, 1)

		assert.NoError(t, err)
		assert.True(t, eligible)
		assert.Empty(t, reason)
	})

	t.Run("user not found", func(t *testing.T) {
		mr, queueClient, syncService, _ := setupSyncServiceTest(t)
		defer mr.Close()
		defer queueClient.Close()

		eligible, reason, err := syncService.GetUserSyncStatus(ctx, 999)

		assert.NoError(t, err)
		assert.False(t, eligible)
		assert.Equal(t, "user not found", reason)
	})

	t.Run("no Strava connection", func(t *testing.T) {
		mr, queueClient, syncService, mockRepo := setupSyncServiceTest(t)
		defer mr.Close()
		defer queueClient.Close()

		spreadsheetID := "test-spreadsheet-123"
		mockRepo.users[2] = &database.User{
			ID:            2,
			Email:         "test2@example.com",
			SpreadsheetID: &spreadsheetID,
			// No Strava token
		}

		eligible, reason, err := syncService.GetUserSyncStatus(ctx, 2)

		assert.NoError(t, err)
		assert.False(t, eligible)
		assert.Equal(t, "strava connection required", reason)
	})

	t.Run("no spreadsheet configured", func(t *testing.T) {
		mr, queueClient, syncService, mockRepo := setupSyncServiceTest(t)
		defer mr.Close()
		defer queueClient.Close()

		mockRepo.users[3] = &database.User{
			ID:                 3,
			Email:              "test3@example.com",
			StravaRefreshToken: []byte("encrypted-strava-token"),
			// No spreadsheet
		}

		eligible, reason, err := syncService.GetUserSyncStatus(ctx, 3)

		assert.NoError(t, err)
		assert.False(t, eligible)
		assert.Equal(t, "spreadsheet configuration required", reason)
	})

	t.Run("database error", func(t *testing.T) {
		mr, queueClient, syncService, mockRepo := setupSyncServiceTest(t)
		defer mr.Close()
		defer queueClient.Close()

		mockRepo.err = sql.ErrConnDone

		eligible, reason, err := syncService.GetUserSyncStatus(ctx, 1)

		assert.Error(t, err)
		assert.False(t, eligible)
		assert.Empty(t, reason)
	})
}