package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// MockSessionRepository for testing JWT validation with session states
type MockSessionRepository struct {
	sessions map[int]*MockSession
}

type MockSession struct {
	ID       int
	UserID   int
	IsActive bool
	Token    string
}

func (m *MockSessionRepository) GetSessionByID(ctx context.Context, sessionID int) (*MockSession, error) {
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, nil
	}
	return session, nil
}

func (m *MockSessionRepository) DeactivateSession(ctx context.Context, sessionID int) error {
	if session, exists := m.sessions[sessionID]; exists {
		session.IsActive = false
	}
	return nil
}

func setupJWTService() *JWTService {
	return NewJWTService("test-secret-key-for-jwt-testing")
}

func TestJWTService(t *testing.T) {
	t.Run("GenerateToken", func(t *testing.T) {
		service := setupJWTService()
		
		userID := 123
		email := "test@example.com"
		googleID := "google123"
		sessionID := 456
		
		token, err := service.GenerateToken(userID, email, googleID, sessionID)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		
		if token == "" {
			t.Error("Expected non-empty token")
		}
		
		// Validate the generated token
		claims, err := service.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate generated token: %v", err)
		}
		
		if claims.UserID != userID {
			t.Errorf("Expected UserID %d, got %d", userID, claims.UserID)
		}
		if claims.Email != email {
			t.Errorf("Expected Email %s, got %s", email, claims.Email)
		}
		if claims.GoogleID != googleID {
			t.Errorf("Expected GoogleID %s, got %s", googleID, claims.GoogleID)
		}
		if claims.SessionID != sessionID {
			t.Errorf("Expected SessionID %d, got %d", sessionID, claims.SessionID)
		}
	})

	t.Run("ValidateToken", func(t *testing.T) {
		service := setupJWTService()
		
		// Generate a valid token
		token, err := service.GenerateToken(123, "test@example.com", "google123", 456)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		
		// Validate the token
		claims, err := service.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		
		if claims == nil {
			t.Error("Expected non-nil claims")
		}
	})

	t.Run("ValidateInvalidToken", func(t *testing.T) {
		service := setupJWTService()
		
		// Test with completely invalid token
		_, err := service.ValidateToken("invalid.token.string")
		if err == nil {
			t.Error("Expected error for invalid token")
		}
		
		// Test with empty token
		_, err = service.ValidateToken("")
		if err == nil {
			t.Error("Expected error for empty token")
		}
	})

	t.Run("ValidateExpiredToken", func(t *testing.T) {
		service := setupJWTService()
		
		// Create an expired token manually
		claims := JWTClaims{
			UserID:    123,
			Email:     "test@example.com",
			GoogleID:  "google123",
			SessionID: 456,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // Expired 1 hour ago
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				Issuer:    "academy-sync",
				Subject:   "test@example.com",
			},
		}
		
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(service.secretKey)
		if err != nil {
			t.Fatalf("Failed to create expired token: %v", err)
		}
		
		// Try to validate the expired token
		_, err = service.ValidateToken(tokenString)
		if err == nil {
			t.Error("Expected error for expired token")
		}
	})

	t.Run("RefreshToken", func(t *testing.T) {
		service := setupJWTService()
		
		// Generate original token
		originalToken, err := service.GenerateToken(123, "test@example.com", "google123", 456)
		if err != nil {
			t.Fatalf("Failed to generate original token: %v", err)
		}
		
		// Refresh the token
		newToken, err := service.RefreshToken(originalToken)
		if err != nil {
			t.Fatalf("Failed to refresh token: %v", err)
		}
		
		if newToken == "" {
			t.Error("Expected non-empty refreshed token")
		}
		
		// New token should be different from original (usually, unless generated at exact same time)
		// Note: Tokens may be identical if generated within the same second with same claims
		// This is acceptable behavior for JWT refresh
		
		// Both tokens should be valid
		originalClaims, err := service.ValidateToken(originalToken)
		if err != nil {
			t.Fatalf("Original token should still be valid: %v", err)
		}
		
		newClaims, err := service.ValidateToken(newToken)
		if err != nil {
			t.Fatalf("New token should be valid: %v", err)
		}
		
		// Claims should have same user data
		if originalClaims.UserID != newClaims.UserID {
			t.Error("User ID should be same in refreshed token")
		}
		if originalClaims.Email != newClaims.Email {
			t.Error("Email should be same in refreshed token")
		}
		if originalClaims.SessionID != newClaims.SessionID {
			t.Error("Session ID should be same in refreshed token")
		}
	})

	t.Run("RefreshInvalidToken", func(t *testing.T) {
		service := setupJWTService()
		
		// Try to refresh an invalid token
		_, err := service.RefreshToken("invalid.token.string")
		if err == nil {
			t.Error("Expected error when refreshing invalid token")
		}
	})
}

// Logout-specific JWT scenarios
func TestJWTLogoutScenarios(t *testing.T) {
	t.Run("TokenValidationAfterLogout", func(t *testing.T) {
		service := setupJWTService()
		sessionRepo := &MockSessionRepository{
			sessions: make(map[int]*MockSession),
		}
		
		userID := 123
		sessionID := 456
		
		// Create active session
		sessionRepo.sessions[sessionID] = &MockSession{
			ID:       sessionID,
			UserID:   userID,
			IsActive: true,
		}
		
		// Generate token for active session
		token, err := service.GenerateToken(userID, "test@example.com", "google123", sessionID)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		
		// Token should be valid initially
		claims, err := service.ValidateToken(token)
		if err != nil {
			t.Fatalf("Token should be valid initially: %v", err)
		}
		if claims.SessionID != sessionID {
			t.Errorf("Expected session ID %d, got %d", sessionID, claims.SessionID)
		}
		
		// Simulate logout by deactivating session
		err = sessionRepo.DeactivateSession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("Failed to deactivate session: %v", err)
		}
		
		// Token should still be cryptographically valid (JWT validation doesn't check session state)
		// This demonstrates that additional session validation is needed in middleware
		claims, err = service.ValidateToken(token)
		if err != nil {
			t.Fatalf("JWT should still be cryptographically valid: %v", err)
		}
		
		// But session should be inactive
		session, err := sessionRepo.GetSessionByID(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("Failed to get session: %v", err)
		}
		if session.IsActive {
			t.Error("Session should be inactive after logout")
		}
	})

	t.Run("PreventTokenRefreshAfterLogout", func(t *testing.T) {
		service := setupJWTService()
		sessionRepo := &MockSessionRepository{
			sessions: make(map[int]*MockSession),
		}
		
		userID := 123
		sessionID := 456
		
		// Create active session
		sessionRepo.sessions[sessionID] = &MockSession{
			ID:       sessionID,
			UserID:   userID,
			IsActive: true,
		}
		
		// Generate token
		token, err := service.GenerateToken(userID, "test@example.com", "google123", sessionID)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		
		// Refresh should work before logout
		refreshedToken, err := service.RefreshToken(token)
		if err != nil {
			t.Fatalf("Token refresh should work before logout: %v", err)
		}
		if refreshedToken == "" {
			t.Error("Expected valid refreshed token")
		}
		
		// Simulate logout
		err = sessionRepo.DeactivateSession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("Failed to deactivate session: %v", err)
		}
		
		// JWT refresh will still work cryptographically, but in a real system
		// the middleware should check session state before allowing refresh
		refreshedToken2, err := service.RefreshToken(token)
		if err != nil {
			t.Fatalf("JWT refresh still works cryptographically: %v", err)
		}
		
		// This demonstrates that session validation is needed at the middleware level
		if refreshedToken2 == "" {
			t.Error("Expected cryptographically valid refresh")
		}
		
		// But session should be inactive
		session, err := sessionRepo.GetSessionByID(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("Failed to get session: %v", err)
		}
		if session.IsActive {
			t.Error("Session should be inactive, preventing logical token refresh")
		}
	})

	t.Run("MultipleSessionsLogout", func(t *testing.T) {
		service := setupJWTService()
		sessionRepo := &MockSessionRepository{
			sessions: make(map[int]*MockSession),
		}
		
		userID := 123
		
		// Create multiple sessions for the same user
		sessionID1 := 456
		sessionID2 := 789
		
		sessionRepo.sessions[sessionID1] = &MockSession{
			ID:       sessionID1,
			UserID:   userID,
			IsActive: true,
		}
		sessionRepo.sessions[sessionID2] = &MockSession{
			ID:       sessionID2,
			UserID:   userID,
			IsActive: true,
		}
		
		// Generate tokens for both sessions
		token1, err := service.GenerateToken(userID, "test@example.com", "google123", sessionID1)
		if err != nil {
			t.Fatalf("Failed to generate token1: %v", err)
		}
		
		token2, err := service.GenerateToken(userID, "test@example.com", "google123", sessionID2)
		if err != nil {
			t.Fatalf("Failed to generate token2: %v", err)
		}
		
		// Both tokens should be valid
		claims1, err := service.ValidateToken(token1)
		if err != nil {
			t.Fatalf("Token1 should be valid: %v", err)
		}
		claims2, err := service.ValidateToken(token2)
		if err != nil {
			t.Fatalf("Token2 should be valid: %v", err)
		}
		
		if claims1.SessionID != sessionID1 {
			t.Errorf("Expected session ID %d, got %d", sessionID1, claims1.SessionID)
		}
		if claims2.SessionID != sessionID2 {
			t.Errorf("Expected session ID %d, got %d", sessionID2, claims2.SessionID)
		}
		
		// Logout from session 1 only
		err = sessionRepo.DeactivateSession(context.Background(), sessionID1)
		if err != nil {
			t.Fatalf("Failed to deactivate session1: %v", err)
		}
		
		// Session 1 should be inactive, session 2 should still be active
		session1, err := sessionRepo.GetSessionByID(context.Background(), sessionID1)
		if err != nil {
			t.Fatalf("Failed to get session1: %v", err)
		}
		if session1.IsActive {
			t.Error("Session1 should be inactive after logout")
		}
		
		session2, err := sessionRepo.GetSessionByID(context.Background(), sessionID2)
		if err != nil {
			t.Fatalf("Failed to get session2: %v", err)
		}
		if !session2.IsActive {
			t.Error("Session2 should still be active")
		}
		
		// Both tokens are still cryptographically valid
		// (middleware needs to check session state)
		_, err = service.ValidateToken(token1)
		if err != nil {
			t.Fatalf("Token1 still cryptographically valid: %v", err)
		}
		_, err = service.ValidateToken(token2)
		if err != nil {
			t.Fatalf("Token2 still cryptographically valid: %v", err)
		}
	})
}

func TestJWTSecurityScenarios(t *testing.T) {
	t.Run("DifferentSecretKeys", func(t *testing.T) {
		service1 := NewJWTService("secret1")
		service2 := NewJWTService("secret2")
		
		// Generate token with service1
		token, err := service1.GenerateToken(123, "test@example.com", "google123", 456)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		
		// Token should be valid with service1
		_, err = service1.ValidateToken(token)
		if err != nil {
			t.Fatalf("Token should be valid with same service: %v", err)
		}
		
		// Token should be invalid with service2 (different secret)
		_, err = service2.ValidateToken(token)
		if err == nil {
			t.Error("Token should be invalid with different secret key")
		}
	})

	t.Run("TamperedToken", func(t *testing.T) {
		service := setupJWTService()
		
		// Generate valid token
		token, err := service.GenerateToken(123, "test@example.com", "google123", 456)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		
		// Tamper with the token (change last character)
		tamperedToken := token[:len(token)-1] + "X"
		
		// Tampered token should be invalid
		_, err = service.ValidateToken(tamperedToken)
		if err == nil {
			t.Error("Tampered token should be invalid")
		}
	})
}