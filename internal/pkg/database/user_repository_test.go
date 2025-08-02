package database

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUsersInProcessingWindow(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Create encryption service (not used in this test)
	encryptor := auth.NewEncryptionService("test-secret-key-32-bytes-long!!!!!")

	// Create repository
	repo := NewUserRepository(db, encryptor)

	// Define the expected query pattern that matches the actual implementation
	expectedQuery := `SELECT id\s+FROM users\s+WHERE automation_enabled = true\s+AND strava_refresh_token IS NOT NULL\s+AND LENGTH\(strava_refresh_token\) > 0\s+AND spreadsheet_id IS NOT NULL\s+AND spreadsheet_id != ''\s+AND EXTRACT\(HOUR FROM \(NOW\(\) AT TIME ZONE timezone\)\) >= 3\s+AND EXTRACT\(HOUR FROM \(NOW\(\) AT TIME ZONE timezone\)\) < 4\s+ORDER BY id`

	tests := []struct {
		name          string
		setupMock     func()
		expectedUsers []int
		expectedError bool
		errorMessage  string
	}{
		{
			name: "returns users in processing window",
			setupMock: func() {
				rows := sqlmock.NewRows([]string{"id"}).
					AddRow(1).
					AddRow(3).
					AddRow(5)

				mock.ExpectQuery(expectedQuery).
					WillReturnRows(rows)
			},
			expectedUsers: []int{1, 3, 5},
			expectedError: false,
		},
		{
			name: "returns empty list when no users in window",
			setupMock: func() {
				rows := sqlmock.NewRows([]string{"id"})

				mock.ExpectQuery(expectedQuery).
					WillReturnRows(rows)
			},
			expectedUsers: nil, // GetUsersInProcessingWindow returns nil for empty result
			expectedError: false,
		},
		{
			name: "handles database error",
			setupMock: func() {
				mock.ExpectQuery(expectedQuery).
					WillReturnError(sql.ErrConnDone)
			},
			expectedUsers: nil,
			expectedError: true,
			errorMessage:  "sql: connection is already closed",
		},
		{
			name: "filters by automation enabled and required fields",
			setupMock: func() {
				rows := sqlmock.NewRows([]string{"id"}).
					AddRow(2).
					AddRow(4)

				mock.ExpectQuery(expectedQuery).
					WillReturnRows(rows)
			},
			expectedUsers: []int{2, 4},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock expectations
			tt.setupMock()

			// Execute the method
			ctx := context.Background()
			users, err := repo.GetUsersInProcessingWindow(ctx)

			// Verify results
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
				if tt.expectedUsers == nil {
					assert.Nil(t, users)
				} else {
					assert.Equal(t, tt.expectedUsers, users)
				}
			}

			// Verify all expectations were met
			err = mock.ExpectationsWereMet()
			assert.NoError(t, err)
		})
	}
}

func TestGetUsersInProcessingWindow_Integration(t *testing.T) {
	// Define the expected query pattern that matches the actual implementation
	expectedQuery := `SELECT id\s+FROM users\s+WHERE automation_enabled = true\s+AND strava_refresh_token IS NOT NULL\s+AND LENGTH\(strava_refresh_token\) > 0\s+AND spreadsheet_id IS NOT NULL\s+AND spreadsheet_id != ''\s+AND EXTRACT\(HOUR FROM \(NOW\(\) AT TIME ZONE timezone\)\) >= 3\s+AND EXTRACT\(HOUR FROM \(NOW\(\) AT TIME ZONE timezone\)\) < 4\s+ORDER BY id`

	// This test demonstrates the timezone calculation logic
	// It would require a real database connection to test properly
	t.Run("timezone calculation", func(t *testing.T) {
		// Create mock database
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		encryptor := auth.NewEncryptionService("test-secret-key-32-bytes-long!!!!!")
		repo := NewUserRepository(db, encryptor)

		// Test that the query correctly filters by timezone
		// Add test users with different timezones and verify filtering
		rows := sqlmock.NewRows([]string{"id"}).
			AddRow(1). // UTC user (3 AM - in window)
			AddRow(3)  // Europe/London user (3 AM - in window)

		mock.ExpectQuery(expectedQuery).
			WillReturnRows(rows)

		ctx := context.Background()
		users, err := repo.GetUsersInProcessingWindow(ctx)
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 3}, users)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestGetUsersInProcessingWindow_RowScanError(t *testing.T) {
	// Define the expected query pattern that matches the actual implementation
	expectedQuery := `SELECT id\s+FROM users\s+WHERE automation_enabled = true\s+AND strava_refresh_token IS NOT NULL\s+AND LENGTH\(strava_refresh_token\) > 0\s+AND spreadsheet_id IS NOT NULL\s+AND spreadsheet_id != ''\s+AND EXTRACT\(HOUR FROM \(NOW\(\) AT TIME ZONE timezone\)\) >= 3\s+AND EXTRACT\(HOUR FROM \(NOW\(\) AT TIME ZONE timezone\)\) < 4\s+ORDER BY id`

	// Test handling of row scan errors
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	encryptor := auth.NewEncryptionService("test-secret-key-32-bytes-long!!!!!")
	repo := NewUserRepository(db, encryptor)

	// Create rows that will cause a scan error (wrong type)
	rows := sqlmock.NewRows([]string{"id"}).
		AddRow("not-an-int") // This will cause a scan error

	mock.ExpectQuery(expectedQuery).
		WillReturnRows(rows)

	ctx := context.Background()
	users, err := repo.GetUsersInProcessingWindow(ctx)

	assert.Error(t, err)
	assert.Nil(t, users)
	assert.Contains(t, err.Error(), "Scan")
}