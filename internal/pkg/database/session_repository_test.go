package database

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

const getSessionByIDQuery = `
		SELECT id, user_id, session_token, user_agent, ip_address,
			   created_at, expires_at, last_used_at, is_active
		FROM user_sessions 
		WHERE id = $1 AND is_active = true AND expires_at > $2
	`

const createSessionQuery = `
		INSERT INTO user_sessions (
			user_id, session_token, user_agent, ip_address, 
			created_at, expires_at, last_used_at, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, last_used_at
	`

const updateSessionTokenQuery = `UPDATE user_sessions SET session_token = $1 WHERE id = $2`

const deactivateSessionQuery = `UPDATE user_sessions SET is_active = false WHERE id = $1`

func setupTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}
	return db, mock
}

func TestGetSessionByID(t *testing.T) {
	t.Run("ValidSession", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()

		repo := NewSessionRepository(db)
		ctx := context.Background()

		// Mock data
		sessionID := 1
		userID := 123
		sessionToken := "test-token-123"
		userAgent := "Test User Agent"
		ipAddress := "127.0.0.1"
		createdAt := time.Now()
		expiresAt := time.Now().Add(24 * time.Hour)
		lastUsedAt := time.Now()
		isActive := true

		// Set up mock expectation for GetSessionByID query
		mock.ExpectQuery(regexp.QuoteMeta(getSessionByIDQuery)).
			WithArgs(sessionID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "user_id", "session_token", "user_agent", "ip_address",
				"created_at", "expires_at", "last_used_at", "is_active",
			}).AddRow(sessionID, userID, sessionToken, userAgent, ipAddress, createdAt, expiresAt, lastUsedAt, isActive))

		// Test GetSessionByID with valid ID
		retrievedSession, err := repo.GetSessionByID(ctx, sessionID)
		if err != nil {
			t.Fatalf("GetSessionByID failed: %v", err)
		}

		if retrievedSession == nil {
			t.Fatal("GetSessionByID returned nil session")
		}

		if retrievedSession.ID != sessionID {
			t.Errorf("Expected session ID %d, got %d", sessionID, retrievedSession.ID)
		}

		if retrievedSession.UserID != userID {
			t.Errorf("Expected user ID %d, got %d", userID, retrievedSession.UserID)
		}

		if retrievedSession.SessionToken != sessionToken {
			t.Errorf("Expected session token %s, got %s", sessionToken, retrievedSession.SessionToken)
		}

		if retrievedSession.IsActive != isActive {
			t.Errorf("Expected is_active %v, got %v", isActive, retrievedSession.IsActive)
		}

		// Verify all expectations were met
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("NonExistentSession", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()

		repo := NewSessionRepository(db)
		ctx := context.Background()

		// Set up mock expectation for non-existent session
		mock.ExpectQuery(regexp.QuoteMeta(getSessionByIDQuery)).
			WithArgs(99999, sqlmock.AnyArg()).
			WillReturnError(sql.ErrNoRows)

		// Test GetSessionByID with non-existent ID
		nonExistentSession, err := repo.GetSessionByID(ctx, 99999)
		if err != nil {
			t.Fatalf("GetSessionByID with non-existent ID should not return error: %v", err)
		}

		if nonExistentSession != nil {
			t.Error("GetSessionByID with non-existent ID should return nil")
		}

		// Verify all expectations were met
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("ExpiredSession", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()

		repo := NewSessionRepository(db)
		ctx := context.Background()

		// Set up mock expectation for expired session (should return no rows because of expires_at check)
		mock.ExpectQuery(regexp.QuoteMeta(getSessionByIDQuery)).
			WithArgs(456, sqlmock.AnyArg()).
			WillReturnError(sql.ErrNoRows)

		// Try to retrieve expired session
		retrievedExpiredSession, err := repo.GetSessionByID(ctx, 456)
		if err != nil {
			t.Fatalf("GetSessionByID with expired session should not return error: %v", err)
		}

		if retrievedExpiredSession != nil {
			t.Error("GetSessionByID with expired session should return nil")
		}

		// Verify all expectations were met
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("InactiveSession", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()

		repo := NewSessionRepository(db)
		ctx := context.Background()

		// Set up mock expectation for inactive session (should return no rows because of is_active check)
		mock.ExpectQuery(regexp.QuoteMeta(getSessionByIDQuery)).
			WithArgs(789, sqlmock.AnyArg()).
			WillReturnError(sql.ErrNoRows)

		// Try to retrieve inactive session
		retrievedInactiveSession, err := repo.GetSessionByID(ctx, 789)
		if err != nil {
			t.Fatalf("GetSessionByID with inactive session should not return error: %v", err)
		}

		if retrievedInactiveSession != nil {
			t.Error("GetSessionByID with inactive session should return nil")
		}

		// Verify all expectations were met
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})

	t.Run("DatabaseError", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()

		repo := NewSessionRepository(db)
		ctx := context.Background()

		// Set up mock expectation for database error
		mock.ExpectQuery(regexp.QuoteMeta(getSessionByIDQuery)).
			WithArgs(123, sqlmock.AnyArg()).
			WillReturnError(sql.ErrConnDone)

		// Test GetSessionByID with database error
		session, err := repo.GetSessionByID(ctx, 123)
		if err == nil {
			t.Error("Expected database error, got nil")
		}

		if session != nil {
			t.Error("Expected nil session on database error")
		}

		// Verify all expectations were met
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})
}

func TestCreateSession(t *testing.T) {
	t.Run("ValidSession", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()

		repo := NewSessionRepository(db)
		ctx := context.Background()

		// Mock data
		userID := 123
		sessionToken := "test-token-123"
		userAgent := stringPtr("Test User Agent")
		ipAddress := stringPtr("127.0.0.1")
		expiresAt := time.Now().Add(24 * time.Hour)
		returnedSessionID := 1
		createdAt := time.Now()
		lastUsedAt := time.Now()

		// Set up mock expectation for CreateSession query
		// Note: CreateSession only returns "id", "created_at", "last_used_at" columns in RETURNING clause
		mock.ExpectQuery(regexp.QuoteMeta(createSessionQuery)).
			WithArgs(userID, sessionToken, userAgent, ipAddress, sqlmock.AnyArg(), expiresAt, sqlmock.AnyArg(), true).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "last_used_at"}).
				AddRow(returnedSessionID, createdAt, lastUsedAt))

		// Test CreateSession
		sessionReq := &CreateSessionRequest{
			UserID:       userID,
			SessionToken: sessionToken,
			UserAgent:    userAgent,
			IPAddress:    ipAddress,
			ExpiresAt:    expiresAt,
		}

		session, err := repo.CreateSession(ctx, sessionReq)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		if session == nil {
			t.Fatal("CreateSession returned nil session")
		}

		if session.ID != returnedSessionID {
			t.Errorf("Expected session ID %d, got %d", returnedSessionID, session.ID)
		}

		if session.UserID != userID {
			t.Errorf("Expected user ID %d, got %d", userID, session.UserID)
		}

		if session.SessionToken != sessionToken {
			t.Errorf("Expected session token %s, got %s", sessionToken, session.SessionToken)
		}

		if session.UserAgent == nil || *session.UserAgent != *userAgent {
			t.Errorf("Expected user agent %v, got %v", userAgent, session.UserAgent)
		}

		if session.IPAddress == nil || *session.IPAddress != *ipAddress {
			t.Errorf("Expected IP address %v, got %v", ipAddress, session.IPAddress)
		}

		if !session.ExpiresAt.Equal(expiresAt) {
			t.Errorf("Expected expires at %v, got %v", expiresAt, session.ExpiresAt)
		}

		if !session.IsActive {
			t.Error("Expected session to be active")
		}

		// Verify the returned timestamps match what was mocked
		if !session.CreatedAt.Equal(createdAt) {
			t.Errorf("Expected created at %v, got %v", createdAt, session.CreatedAt)
		}

		if !session.LastUsedAt.Equal(lastUsedAt) {
			t.Errorf("Expected last used at %v, got %v", lastUsedAt, session.LastUsedAt)
		}

		// Verify all expectations were met
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})
}

func TestUpdateSessionToken(t *testing.T) {
	t.Run("ValidUpdate", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()

		repo := NewSessionRepository(db)
		ctx := context.Background()

		sessionID := 1
		newToken := "new-token-456"

		// Set up mock expectation for UpdateSessionToken query
		mock.ExpectExec(regexp.QuoteMeta(updateSessionTokenQuery)).
			WithArgs(newToken, sessionID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Test UpdateSessionToken
		err := repo.UpdateSessionToken(ctx, sessionID, newToken)
		if err != nil {
			t.Fatalf("UpdateSessionToken failed: %v", err)
		}

		// Verify all expectations were met
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})
}

func TestDeactivateSession(t *testing.T) {
	t.Run("ValidDeactivation", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()

		repo := NewSessionRepository(db)
		ctx := context.Background()

		sessionID := 1

		// Set up mock expectation for DeactivateSession query
		mock.ExpectExec(regexp.QuoteMeta(deactivateSessionQuery)).
			WithArgs(sessionID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Test DeactivateSession
		err := repo.DeactivateSession(ctx, sessionID)
		if err != nil {
			t.Fatalf("DeactivateSession failed: %v", err)
		}

		// Verify all expectations were met
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %s", err)
		}
	})
}

func stringPtr(s string) *string {
	return &s
}
