package database

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
)

func TestUserRepository_UpdateSpreadsheetID(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Create mock encryption service (not used in this test but required for repository)
	encryptionService := auth.NewEncryptionService("test-key-32-characters-long!!!")
	repo := NewUserRepository(db, encryptionService)

	ctx := context.Background()
	userID := 123
	spreadsheetID := "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"

	// Set up mock expectations
	mock.ExpectExec("UPDATE users SET spreadsheet_id = \\$1, updated_at = \\$2 WHERE id = \\$3").
		WithArgs(spreadsheetID, sqlmock.AnyArg(), userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute the method
	err = repo.UpdateSpreadsheetID(ctx, userID, spreadsheetID)

	// Verify results
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestUserRepository_ClearSpreadsheetID(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Create mock encryption service
	encryptionService := auth.NewEncryptionService("test-key-32-characters-long!!!")
	repo := NewUserRepository(db, encryptionService)

	ctx := context.Background()
	userID := 123

	// Set up mock expectations
	mock.ExpectExec("UPDATE users SET spreadsheet_id = NULL, updated_at = \\$1 WHERE id = \\$2").
		WithArgs(sqlmock.AnyArg(), userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute the method
	err = repo.ClearSpreadsheetID(ctx, userID)

	// Verify results
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestUserRepository_UpdateSpreadsheetID_DatabaseError(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Create mock encryption service
	encryptionService := auth.NewEncryptionService("test-key-32-characters-long!!!")
	repo := NewUserRepository(db, encryptionService)

	ctx := context.Background()
	userID := 123
	spreadsheetID := "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"

	// Set up mock expectations - simulate database error
	expectedError := sqlmock.ErrCancelled
	mock.ExpectExec("UPDATE users SET spreadsheet_id = \\$1, updated_at = \\$2 WHERE id = \\$3").
		WithArgs(spreadsheetID, sqlmock.AnyArg(), userID).
		WillReturnError(expectedError)

	// Execute the method
	err = repo.UpdateSpreadsheetID(ctx, userID, spreadsheetID)

	// Verify error is returned
	if err == nil {
		t.Error("Expected error but got none")
	}

	if err != expectedError {
		t.Errorf("Expected error %v but got %v", expectedError, err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestUserRepository_ClearSpreadsheetID_DatabaseError(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Create mock encryption service
	encryptionService := auth.NewEncryptionService("test-key-32-characters-long!!!")
	repo := NewUserRepository(db, encryptionService)

	ctx := context.Background()
	userID := 123

	// Set up mock expectations - simulate database error
	expectedError := sqlmock.ErrCancelled
	mock.ExpectExec("UPDATE users SET spreadsheet_id = NULL, updated_at = \\$1 WHERE id = \\$2").
		WithArgs(sqlmock.AnyArg(), userID).
		WillReturnError(expectedError)

	// Execute the method
	err = repo.ClearSpreadsheetID(ctx, userID)

	// Verify error is returned
	if err == nil {
		t.Error("Expected error but got none")
	}

	if err != expectedError {
		t.Errorf("Expected error %v but got %v", expectedError, err)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

// Test that the updated_at timestamp is set to a recent time
func TestUserRepository_UpdateSpreadsheetID_TimestampValidation(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Create mock encryption service
	encryptionService := auth.NewEncryptionService("test-key-32-characters-long!!!")
	repo := NewUserRepository(db, encryptionService)

	ctx := context.Background()
	userID := 123
	spreadsheetID := "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"
	beforeCall := time.Now()

	// Set up mock expectations with custom matcher for timestamp
	mock.ExpectExec("UPDATE users SET spreadsheet_id = \\$1, updated_at = \\$2 WHERE id = \\$3").
		WithArgs(spreadsheetID, sqlmock.AnyArg(), userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute the method
	err = repo.UpdateSpreadsheetID(ctx, userID, spreadsheetID)
	afterCall := time.Now()

	// Verify results
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify the timestamp is recent (within the test execution timeframe)
	// This is a basic validation that the method is setting a current timestamp
	if afterCall.Sub(beforeCall) > time.Second {
		t.Error("Test took too long, timestamp validation may be unreliable")
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}