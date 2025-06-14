package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// MockSessionRepositoryForIntegration provides a complete mock for integration testing
type MockSessionRepositoryForIntegration struct {
	sessions map[int]*MockSessionData
}

type MockSessionData struct {
	ID           int
	UserID       int
	SessionToken string
	IsActive     bool
}

func (m *MockSessionRepositoryForIntegration) DeactivateSession(ctx context.Context, sessionID int) error {
	if session, exists := m.sessions[sessionID]; exists {
		session.IsActive = false
	}
	return nil
}

func (m *MockSessionRepositoryForIntegration) GetSessionByID(ctx context.Context, sessionID int) (*MockSessionData, error) {
	session, exists := m.sessions[sessionID]
	if !exists || !session.IsActive {
		return nil, nil
	}
	return session, nil
}

func (m *MockSessionRepositoryForIntegration) CreateSession(ctx context.Context, req interface{}) (*MockSessionData, error) {
	return nil, nil
}

func (m *MockSessionRepositoryForIntegration) UpdateSessionToken(ctx context.Context, sessionID int, token string) error {
	return nil
}

func (m *MockSessionRepositoryForIntegration) DeactivateAllUserSessions(ctx context.Context, userID int) error {
	return nil
}

func (m *MockSessionRepositoryForIntegration) CleanupExpiredSessions(ctx context.Context) error {
	return nil
}

// TestCompleteLogoutFlow tests the entire logout flow from HTTP request to response
func TestCompleteLogoutFlow(t *testing.T) {
	t.Run("EndToEndLogoutFlow", func(t *testing.T) {
		// This test demonstrates the complete logout flow:
		// 1. User has valid session
		// 2. User calls logout endpoint
		// 3. Session is deactivated in database
		// 4. Session cookie is cleared
		// 5. Success response is returned
		
		// Setup JWT service
		jwtService := auth.NewJWTService("test-secret-key")
		
		// Setup mock session repository
		sessionRepo := &MockSessionRepositoryForIntegration{
			sessions: make(map[int]*MockSessionData),
		}
		
		userID := 123
		sessionID := 456
		
		// Create active session
		sessionRepo.sessions[sessionID] = &MockSessionData{
			ID:           sessionID,
			UserID:       userID,
			SessionToken: "active-session-token",
			IsActive:     true,
		}
		
		// Create testable auth handler
		handler := &TestableAuthHandler{
			AuthHandler: &AuthHandler{
				oauthService:      nil,
				jwtService:        jwtService,
				userRepository:    nil,
				sessionRepository: nil, // Will use mock function
				frontendURL:       "http://localhost:3000",
				isDevelopment:     true,
				logger:            logger.New("test"),
			},
		}
		
		// Set up mock session deactivation
		handler.mockDeactivateSession = func(ctx context.Context, id int) error {
			return sessionRepo.DeactivateSession(ctx, id)
		}
		
		// Phase 1: Verify session is active before logout
		session, err := sessionRepo.GetSessionByID(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("Failed to get session: %v", err)
		}
		if session == nil {
			t.Fatal("Session should exist before logout")
		}
		if !session.IsActive {
			t.Error("Session should be active before logout")
		}
		
		// Phase 2: Create logout request with context
		req := httptest.NewRequest("POST", "/api/auth/logout", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		ctx = context.WithValue(ctx, middleware.SessionIDKey, sessionID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		// Phase 3: Call logout handler
		handler.testLogout(w, req)
		
		// Phase 4: Verify HTTP response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		// Verify response content type
		if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}
		
		// Verify response body
		var response map[string]string
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response body: %v", err)
		}
		
		if response["message"] != "Logged out successfully" {
			t.Errorf("Expected success message, got '%s'", response["message"])
		}
		
		// Phase 5: Verify session was deactivated
		session, err = sessionRepo.GetSessionByID(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("Failed to get session after logout: %v", err)
		}
		// GetSessionByID returns nil for inactive sessions
		if session != nil {
			t.Error("Session should be deactivated after logout")
		}
		
		// Verify in sessions map directly
		if sessionRepo.sessions[sessionID].IsActive {
			t.Error("Session should be inactive in repository")
		}
		
		// Phase 6: Verify cookie was cleared
		cookies := w.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "session_token" {
				sessionCookie = cookie
				break
			}
		}
		
		if sessionCookie == nil {
			t.Error("Expected session_token cookie to be set for clearing")
		} else {
			if sessionCookie.Value != "" {
				t.Errorf("Expected empty cookie value, got '%s'", sessionCookie.Value)
			}
			if sessionCookie.MaxAge != -1 {
				t.Errorf("Expected MaxAge -1, got %d", sessionCookie.MaxAge)
			}
			if !sessionCookie.HttpOnly {
				t.Error("Expected HttpOnly cookie")
			}
		}
	})

	t.Run("LogoutWithoutActiveSession", func(t *testing.T) {
		// Test logout behavior when session doesn't exist or is already inactive
		
		sessionRepo := &MockSessionRepositoryForIntegration{
			sessions: make(map[int]*MockSessionData),
		}
		
		userID := 123
		sessionID := 999 // Non-existent session
		
		handler := &TestableAuthHandler{
			AuthHandler: &AuthHandler{
				oauthService:      nil,
				jwtService:        nil,
				userRepository:    nil,
				sessionRepository: nil,
				frontendURL:       "http://localhost:3000",
				isDevelopment:     true,
				logger:            logger.New("test"),
			},
		}
		
		// Mock will be called but session doesn't exist
		sessionDeactivationCalled := false
		handler.mockDeactivateSession = func(ctx context.Context, id int) error {
			sessionDeactivationCalled = true
			return sessionRepo.DeactivateSession(ctx, id)
		}
		
		// Create logout request
		req := httptest.NewRequest("POST", "/api/auth/logout", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		ctx = context.WithValue(ctx, middleware.SessionIDKey, sessionID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		// Call logout handler
		handler.testLogout(w, req)
		
		// Should still succeed (logout is idempotent)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		// Deactivation should have been called
		if !sessionDeactivationCalled {
			t.Error("Expected session deactivation to be called")
		}
		
		// Cookie should still be cleared
		cookies := w.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "session_token" {
				sessionCookie = cookie
				break
			}
		}
		
		if sessionCookie == nil {
			t.Error("Expected session_token cookie to be cleared")
		}
	})

	t.Run("LogoutInProductionMode", func(t *testing.T) {
		// Test that logout works correctly in production mode with different cookie settings
		
		sessionRepo := &MockSessionRepositoryForIntegration{
			sessions: map[int]*MockSessionData{
				456: {
					ID:           456,
					UserID:       123,
					SessionToken: "prod-session-token",
					IsActive:     true,
				},
			},
		}
		
		handler := &TestableAuthHandler{
			AuthHandler: &AuthHandler{
				oauthService:      nil,
				jwtService:        nil,
				userRepository:    nil,
				sessionRepository: nil,
				frontendURL:       "https://app.example.com",
				isDevelopment:     false, // Production mode
				logger:            logger.New("test"),
			},
		}
		
		handler.mockDeactivateSession = func(ctx context.Context, id int) error {
			return sessionRepo.DeactivateSession(ctx, id)
		}
		
		// Create logout request
		req := httptest.NewRequest("POST", "/api/auth/logout", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, 123)
		ctx = context.WithValue(ctx, middleware.SessionIDKey, 456)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		// Call logout handler
		handler.testLogout(w, req)
		
		// Verify response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		// Verify production cookie settings
		cookies := w.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "session_token" {
				sessionCookie = cookie
				break
			}
		}
		
		if sessionCookie == nil {
			t.Error("Expected session_token cookie")
		} else {
			// In production: empty domain, secure=true
			if sessionCookie.Domain != "" {
				t.Errorf("Expected empty domain in production, got '%s'", sessionCookie.Domain)
			}
			if !sessionCookie.Secure {
				t.Error("Expected secure cookie in production")
			}
			if sessionCookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("Expected SameSite Lax, got %v", sessionCookie.SameSite)
			}
		}
	})
}

// TestLogoutFlowErrorHandling tests error scenarios in the logout flow
func TestLogoutFlowErrorHandling(t *testing.T) {
	t.Run("DatabaseErrorDuringLogout", func(t *testing.T) {
		// Test that logout still succeeds even if database operation fails
		
		handler := &TestableAuthHandler{
			AuthHandler: &AuthHandler{
				oauthService:      nil,
				jwtService:        nil,
				userRepository:    nil,
				sessionRepository: nil,
				frontendURL:       "http://localhost:3000",
				isDevelopment:     true,
				logger:            logger.New("test"),
			},
		}
		
		// Mock database error
		handler.mockDeactivateSession = func(ctx context.Context, id int) error {
			return &DatabaseError{Message: "Database connection failed"}
		}
		
		// Create logout request
		req := httptest.NewRequest("POST", "/api/auth/logout", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, 123)
		ctx = context.WithValue(ctx, middleware.SessionIDKey, 456)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		// Call logout handler
		handler.testLogout(w, req)
		
		// Should still succeed (logout continues despite DB error)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 even with DB error, got %d", w.Code)
		}
		
		// Response should still indicate success
		var response map[string]string
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		if response["message"] != "Logged out successfully" {
			t.Errorf("Expected success message even with DB error, got '%s'", response["message"])
		}
		
		// Cookie should still be cleared
		cookies := w.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "session_token" {
				sessionCookie = cookie
				break
			}
		}
		
		if sessionCookie == nil {
			t.Error("Expected session_token cookie to be cleared even with DB error")
		}
	})

	t.Run("LogoutWithoutSessionContext", func(t *testing.T) {
		// Test logout when session ID is not in context
		
		handler := &TestableAuthHandler{
			AuthHandler: &AuthHandler{
				oauthService:      nil,
				jwtService:        nil,
				userRepository:    nil,
				sessionRepository: nil,
				frontendURL:       "http://localhost:3000",
				isDevelopment:     true,
				logger:            logger.New("test"),
			},
		}
		
		sessionDeactivationCalled := false
		handler.mockDeactivateSession = func(ctx context.Context, id int) error {
			sessionDeactivationCalled = true
			return nil
		}
		
		// Create logout request without session ID in context
		req := httptest.NewRequest("POST", "/api/auth/logout", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, 123)
		// No session ID in context
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		// Call logout handler
		handler.testLogout(w, req)
		
		// Should still succeed
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		// Session deactivation should not be called
		if sessionDeactivationCalled {
			t.Error("Session deactivation should not be called when no session ID")
		}
		
		// Cookie should still be cleared
		cookies := w.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "session_token" {
				sessionCookie = cookie
				break
			}
		}
		
		if sessionCookie == nil {
			t.Error("Expected session_token cookie to be cleared even without session context")
		}
	})
}

// Helper error type for testing
type DatabaseError struct {
	Message string
}

func (e *DatabaseError) Error() string {
	return "Database error: " + e.Message
}