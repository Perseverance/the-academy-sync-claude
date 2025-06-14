package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func setupTestDB(t *testing.T) *sql.DB {
	// This would normally use a test database connection
	// For now, we'll skip integration tests if no DB is available
	t.Skip("Skipping database integration test - requires test database setup")
	return nil
}

func TestGetSessionByID(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := NewSessionRepository(db)
	ctx := context.Background()

	// Create a test session first
	sessionReq := &CreateSessionRequest{
		UserID:       1,
		SessionToken: "test-token-123",
		UserAgent:    stringPtr("Test User Agent"),
		IPAddress:    stringPtr("127.0.0.1"),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}

	session, err := repo.CreateSession(ctx, sessionReq)
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	// Test GetSessionByID with valid ID
	retrievedSession, err := repo.GetSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID failed: %v", err)
	}

	if retrievedSession == nil {
		t.Fatal("GetSessionByID returned nil session")
	}

	if retrievedSession.ID != session.ID {
		t.Errorf("Expected session ID %d, got %d", session.ID, retrievedSession.ID)
	}

	if retrievedSession.UserID != session.UserID {
		t.Errorf("Expected user ID %d, got %d", session.UserID, retrievedSession.UserID)
	}

	if retrievedSession.SessionToken != session.SessionToken {
		t.Errorf("Expected session token %s, got %s", session.SessionToken, retrievedSession.SessionToken)
	}

	// Test GetSessionByID with non-existent ID
	nonExistentSession, err := repo.GetSessionByID(ctx, 99999)
	if err != nil {
		t.Fatalf("GetSessionByID with non-existent ID should not return error: %v", err)
	}

	if nonExistentSession != nil {
		t.Error("GetSessionByID with non-existent ID should return nil")
	}

	// Test GetSessionByID with expired session
	// Create an expired session
	expiredSessionReq := &CreateSessionRequest{
		UserID:       1,
		SessionToken: "expired-token-123",
		UserAgent:    stringPtr("Test User Agent"),
		IPAddress:    stringPtr("127.0.0.1"),
		ExpiresAt:    time.Now().Add(-1 * time.Hour), // Already expired
	}

	expiredSession, err := repo.CreateSession(ctx, expiredSessionReq)
	if err != nil {
		t.Fatalf("Failed to create expired test session: %v", err)
	}

	// Try to retrieve expired session
	retrievedExpiredSession, err := repo.GetSessionByID(ctx, expiredSession.ID)
	if err != nil {
		t.Fatalf("GetSessionByID with expired session should not return error: %v", err)
	}

	if retrievedExpiredSession != nil {
		t.Error("GetSessionByID with expired session should return nil")
	}
}

func stringPtr(s string) *string {
	return &s
}
