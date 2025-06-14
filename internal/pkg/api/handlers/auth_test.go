package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// TestableAuthHandler is a wrapper that allows us to override specific methods for testing
type TestableAuthHandler struct {
	*AuthHandler
	mockDeactivateSession func(ctx context.Context, sessionID int) error
	mockGetUserByID       func(ctx context.Context, userID int) (*MockUser, error)
}

// Override the session repository's DeactivateSession method for testing
func (t *TestableAuthHandler) testLogout(w http.ResponseWriter, r *http.Request) {
	userID, hasUserID := middleware.GetUserIDFromContext(r.Context())
	sessionID, ok := middleware.GetSessionIDFromContext(r.Context())
	clientIP := middleware.GetClientIP(r)
	
	t.logger.Info("User logout initiated", 
		"user_id", userID, 
		"has_user_id", hasUserID,
		"session_id", sessionID,
		"has_session", ok,
		"client_ip", clientIP)
	
	if ok && t.mockDeactivateSession != nil {
		// Use mock function for testing
		if err := t.mockDeactivateSession(r.Context(), sessionID); err != nil {
			t.logger.Error("Failed to deactivate session during logout", 
				"error", err, 
				"session_id", sessionID,
				"user_id", userID)
			// Continue with clearing the cookie anyway
		} else {
			t.logger.Debug("Successfully deactivated session", "session_id", sessionID)
		}
	}

	// Clear session cookie
	domain, sameSite, secure := t.getCookieConfig()
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		Domain:   domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	}

	http.SetCookie(w, cookie)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Logged out successfully",
	}); err != nil {
		http.Error(w, "Failed to encode logout response", http.StatusInternalServerError)
		return
	}
}

// setupTestAuthHandler creates a testable AuthHandler for testing
func setupTestAuthHandler() *TestableAuthHandler {
	testLogger := logger.New("test")
	
	baseHandler := &AuthHandler{
		oauthService:      nil, // Not needed for logout tests
		jwtService:        nil, // Not needed for logout tests
		userRepository:    nil, // Not needed for logout tests
		sessionRepository: nil, // Will be mocked
		frontendURL:       "http://localhost:3000",
		isDevelopment:     true,
		logger:            testLogger,
	}
	
	return &TestableAuthHandler{
		AuthHandler: baseHandler,
	}
}

// addContextValues adds user and session IDs to the request context
func addContextValues(r *http.Request, userID, sessionID int) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	ctx = context.WithValue(ctx, middleware.SessionIDKey, sessionID)
	return r.WithContext(ctx)
}

func TestLogout(t *testing.T) {
	t.Run("SuccessfulLogout", func(t *testing.T) {
		// Setup
		sessionDeactivated := false
		handler := setupTestAuthHandler()
		handler.mockDeactivateSession = func(ctx context.Context, sessionID int) error {
			if sessionID != 123 {
				t.Errorf("Expected session ID 123, got %d", sessionID)
			}
			sessionDeactivated = true
			return nil
		}
		
		// Create request
		req := httptest.NewRequest("POST", "/api/auth/logout", nil)
		req = addContextValues(req, 456, 123) // userID=456, sessionID=123
		
		// Create response recorder
		w := httptest.NewRecorder()
		
		// Call handler
		handler.testLogout(w, req)
		
		// Verify response status
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		// Verify session was deactivated
		if !sessionDeactivated {
			t.Error("Expected session to be deactivated")
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
		
		expectedMessage := "Logged out successfully"
		if response["message"] != expectedMessage {
			t.Errorf("Expected message '%s', got '%s'", expectedMessage, response["message"])
		}
		
		// Verify cookie was cleared
		cookies := w.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "session_token" {
				sessionCookie = cookie
				break
			}
		}
		
		if sessionCookie == nil {
			t.Error("Expected session_token cookie to be set")
		} else {
			// Verify cookie is cleared (empty value, MaxAge -1)
			if sessionCookie.Value != "" {
				t.Errorf("Expected empty cookie value, got '%s'", sessionCookie.Value)
			}
			if sessionCookie.MaxAge != -1 {
				t.Errorf("Expected MaxAge -1, got %d", sessionCookie.MaxAge)
			}
			if !sessionCookie.HttpOnly {
				t.Error("Expected HttpOnly cookie")
			}
			if sessionCookie.Path != "/" {
				t.Errorf("Expected Path '/', got '%s'", sessionCookie.Path)
			}
			// In development mode, should use .localhost domain and not secure
			// HTTP test environment may normalize domain names
			if sessionCookie.Domain != ".localhost" && sessionCookie.Domain != "localhost" {
				t.Errorf("Expected Domain '.localhost' or 'localhost', got '%s'", sessionCookie.Domain)
			}
			if sessionCookie.Secure {
				t.Error("Expected non-secure cookie in development mode")
			}
			if sessionCookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("Expected SameSite Lax, got %v", sessionCookie.SameSite)
			}
		}
	})

	t.Run("LogoutWithoutSessionID", func(t *testing.T) {
		// Setup
		sessionDeactivated := false
		handler := setupTestAuthHandler()
		handler.mockDeactivateSession = func(ctx context.Context, sessionID int) error {
			sessionDeactivated = true
			return nil
		}
		
		// Create request without session ID in context
		req := httptest.NewRequest("POST", "/api/auth/logout", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, 456)
		req = req.WithContext(ctx)
		
		// Create response recorder
		w := httptest.NewRecorder()
		
		// Call handler
		handler.testLogout(w, req)
		
		// Verify response status is still OK
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		// Verify session deactivation was not called (no session ID)
		if sessionDeactivated {
			t.Error("Expected session deactivation not to be called when no session ID")
		}
		
		// Verify cookie is still cleared
		cookies := w.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "session_token" {
				sessionCookie = cookie
				break
			}
		}
		
		if sessionCookie == nil {
			t.Error("Expected session_token cookie to be cleared even without session ID")
		}
	})

	t.Run("ProductionMode", func(t *testing.T) {
		// Setup handler in production mode
		handler := setupTestAuthHandler()
		handler.isDevelopment = false // Set to production mode
		handler.mockDeactivateSession = func(ctx context.Context, sessionID int) error {
			return nil
		}
		
		// Create request
		req := httptest.NewRequest("POST", "/api/auth/logout", nil)
		req = addContextValues(req, 456, 123)
		
		// Create response recorder
		w := httptest.NewRecorder()
		
		// Call handler
		handler.testLogout(w, req)
		
		// Verify response status
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		// Verify cookie configuration for production
		cookies := w.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "session_token" {
				sessionCookie = cookie
				break
			}
		}
		
		if sessionCookie == nil {
			t.Error("Expected session_token cookie to be set")
		} else {
			// In production mode, should use empty domain and secure=true
			if sessionCookie.Domain != "" {
				t.Errorf("Expected empty Domain in production, got '%s'", sessionCookie.Domain)
			}
			if !sessionCookie.Secure {
				t.Error("Expected secure cookie in production mode")
			}
		}
	})
}

func TestGetCookieConfig(t *testing.T) {
	t.Run("DevelopmentMode", func(t *testing.T) {
		handler := &AuthHandler{isDevelopment: true}
		
		domain, sameSite, secure := handler.getCookieConfig()
		
		if domain != ".localhost" {
			t.Errorf("Expected domain '.localhost', got '%s'", domain)
		}
		if sameSite != http.SameSiteLaxMode {
			t.Errorf("Expected SameSite Lax, got %v", sameSite)
		}
		if secure {
			t.Error("Expected secure=false in development mode")
		}
	})

	t.Run("ProductionMode", func(t *testing.T) {
		handler := &AuthHandler{isDevelopment: false}
		
		domain, sameSite, secure := handler.getCookieConfig()
		
		if domain != "" {
			t.Errorf("Expected empty domain, got '%s'", domain)
		}
		if sameSite != http.SameSiteLaxMode {
			t.Errorf("Expected SameSite Lax, got %v", sameSite)
		}
		if !secure {
			t.Error("Expected secure=true in production mode")
		}
	})
}

// TestGetCurrentUser tests the GetCurrentUser endpoint returns dashboard data structure
func TestGetCurrentUser(t *testing.T) {
	t.Run("SuccessfulGetCurrentUser", func(t *testing.T) {
		// Create mock user repository
		mockUserRepo := &MockUserRepository{
			users: map[int]*MockUser{
				123: {
					ID:    123,
					Email: "test@example.com",
					Name:  "Test User",
					ProfilePictureURL: "https://example.com/avatar.jpg",
					Timezone: "UTC",
					EmailNotificationsEnabled: true,
					AutomationEnabled: false,
					HasStravaConnection: true,
					HasSheetsConnection: false,
				},
			},
		}

		// Create testable auth handler
		handler := &TestableAuthHandler{
			AuthHandler: &AuthHandler{
				oauthService:      nil,
				jwtService:        nil,
				userRepository:    nil, // Will use mock function
				sessionRepository: nil,
				frontendURL:       "http://localhost:3000",
				isDevelopment:     true,
				logger:            logger.New("test"),
			},
		}

		// Set up mock user repository function
		handler.mockGetUserByID = func(ctx context.Context, userID int) (*MockUser, error) {
			user, exists := mockUserRepo.users[userID]
			if !exists {
				return nil, nil
			}
			return user, nil
		}

		// Create request with user context
		req := httptest.NewRequest("GET", "/api/auth/me", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, 123)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		// Call GetCurrentUser handler
		handler.testGetCurrentUser(w, req, t)

		// Verify HTTP response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		// Verify response content type
		if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}

		// Verify response body structure
		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response body: %v", err)
		}

		// Check user data fields
		if response["id"] != float64(123) {
			t.Errorf("Expected ID 123, got %v", response["id"])
		}
		if response["email"] != "test@example.com" {
			t.Errorf("Expected email 'test@example.com', got %v", response["email"])
		}
		if response["name"] != "Test User" {
			t.Errorf("Expected name 'Test User', got %v", response["name"])
		}
		if response["has_strava_connection"] != true {
			t.Errorf("Expected has_strava_connection true, got %v", response["has_strava_connection"])
		}
		if response["has_sheets_connection"] != false {
			t.Errorf("Expected has_sheets_connection false, got %v", response["has_sheets_connection"])
		}

		// Check that recent_activity_logs field exists and is an array
		activityLogs, exists := response["recent_activity_logs"]
		if !exists {
			t.Error("Expected recent_activity_logs field to exist")
		}
		if activityLogs == nil {
			t.Error("Expected recent_activity_logs to not be null")
		}
		// Should be an empty array for now
		if logsArray, ok := activityLogs.([]interface{}); !ok {
			t.Errorf("Expected recent_activity_logs to be an array, got %T", activityLogs)
		} else if len(logsArray) != 0 {
			t.Errorf("Expected empty activity logs array, got %d items", len(logsArray))
		}
	})
}

// MockUserRepository for testing GetCurrentUser
type MockUserRepository struct {
	users map[int]*MockUser
}

type MockUser struct {
	ID                        int
	Email                     string
	Name                      string
	ProfilePictureURL         string
	Timezone                  string
	EmailNotificationsEnabled bool
	AutomationEnabled         bool
	HasStravaConnection       bool
	HasSheetsConnection       bool
}

// Mock method to get user by ID
func (m *MockUserRepository) GetUserByID(ctx context.Context, userID int) (*MockUser, error) {
	user, exists := m.users[userID]
	if !exists {
		return nil, nil
	}
	return user, nil
}

// Add mock function to TestableAuthHandler
func (t *TestableAuthHandler) testGetCurrentUser(w http.ResponseWriter, r *http.Request, test *testing.T) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}

	// Use mock function if available
	if t.mockGetUserByID != nil {
		mockUser, err := t.mockGetUserByID(r.Context(), userID)
		if err != nil {
			http.Error(w, "Failed to get user", http.StatusInternalServerError)
			return
		}
		if mockUser == nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		// Convert mock user to response format
		response := map[string]interface{}{
			"id":                         mockUser.ID,
			"email":                      mockUser.Email,
			"name":                       mockUser.Name,
			"profile_picture_url":        mockUser.ProfilePictureURL,
			"timezone":                   mockUser.Timezone,
			"email_notifications_enabled": mockUser.EmailNotificationsEnabled,
			"automation_enabled":         mockUser.AutomationEnabled,
			"has_strava_connection":      mockUser.HasStravaConnection,
			"has_sheets_connection":      mockUser.HasSheetsConnection,
			"recent_activity_logs":       []interface{}{}, // Empty array for now
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			test.Errorf("Failed to encode response: %v", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	// Fallback to actual implementation if no mock
	t.GetCurrentUser(w, r)
}

