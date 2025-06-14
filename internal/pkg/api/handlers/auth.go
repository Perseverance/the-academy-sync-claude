package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	oauthService      *auth.OAuthService
	jwtService        *auth.JWTService
	userRepository    *database.UserRepository
	sessionRepository *database.SessionRepository
	frontendURL       string
	isDevelopment     bool
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(
	oauthService *auth.OAuthService,
	jwtService *auth.JWTService,
	userRepository *database.UserRepository,
	sessionRepository *database.SessionRepository,
	frontendURL string,
	isDevelopment bool,
) *AuthHandler {
	return &AuthHandler{
		oauthService:      oauthService,
		jwtService:        jwtService,
		userRepository:    userRepository,
		sessionRepository: sessionRepository,
		frontendURL:       frontendURL,
		isDevelopment:     isDevelopment,
	}
}

// getCookieConfig returns appropriate cookie configuration for the environment
func (h *AuthHandler) getCookieConfig() (domain string, sameSite http.SameSite, secure bool) {
	if h.isDevelopment {
		// Development: Use .localhost domain to share cookies across ports
		// SameSite=Lax allows cookies in top-level navigation (OAuth redirects)
		return ".localhost", http.SameSiteLaxMode, false
	}
	// Production: Use secure settings
	return "", http.SameSiteLaxMode, true
}

// generateSecureState generates a cryptographically secure random state for OAuth CSRF protection
func generateSecureState() (string, error) {
	// Generate 16 bytes (128 bits) of cryptographically secure random data
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate secure random state: %w", err)
	}
	
	// Encode as base64 URL-safe string with "oauth-" prefix for identification
	return "oauth-" + base64.URLEncoding.EncodeToString(randomBytes), nil
}

// GoogleAuthURL generates and returns the Google OAuth authorization URL
func (h *AuthHandler) GoogleAuthURL(w http.ResponseWriter, r *http.Request) {
	// Generate a cryptographically secure state parameter for CSRF protection
	state, err := generateSecureState()
	if err != nil {
		http.Error(w, "Failed to generate secure state", http.StatusInternalServerError)
		return
	}
	
	// Store state in session cookie for validation
	domain, sameSite, secure := h.getCookieConfig()
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		Domain:   domain,
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})

	authURL := h.oauthService.GetAuthURL(state)

	response := map[string]string{
		"auth_url": authURL,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GoogleCallback handles the OAuth callback from Google
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Validate state parameter exists
	stateParam := r.URL.Query().Get("state")
	if stateParam == "" {
		http.Error(w, "Missing state parameter", http.StatusBadRequest)
		return
	}

	// Validate state cookie exists and matches (required for security)
	stateCookie, cookieErr := r.Cookie("oauth_state")
	if cookieErr != nil {
		// In production, state cookie is required for CSRF protection
		if !h.isDevelopment {
			http.Error(w, "Missing state cookie - CSRF protection required", http.StatusBadRequest)
			return
		}
		// In development, allow fallback validation for direct callback testing
		if !strings.HasPrefix(stateParam, "oauth-") {
			http.Error(w, "Invalid state parameter format", http.StatusBadRequest)
			return
		}
	} else {
		// Cookie exists, must match exactly
		if stateCookie.Value != stateParam {
			http.Error(w, "Invalid state parameter - CSRF protection failed", http.StatusBadRequest)
			return
		}
	}

	// Clear the state cookie if it exists
	if cookieErr == nil {
		domain, sameSite, secure := h.getCookieConfig()
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    "",
			Path:     "/",
			Domain:   domain,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secure,
			SameSite: sameSite,
		})
	}

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := h.oauthService.ExchangeCodeForToken(context.Background(), code)
	if err != nil {
		http.Error(w, "Failed to exchange code for token", http.StatusInternalServerError)
		return
	}

	// Get user info from Google
	userInfo, err := h.oauthService.GetUserInfo(context.Background(), token)
	if err != nil {
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	// Check if user already exists
	existingUser, err := h.userRepository.GetUserByGoogleID(r.Context(), userInfo.ID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var user *database.User

	if existingUser != nil {
		// Update existing user's tokens and last login atomically
		user = existingUser
		updateReq := &database.UpdateUserTokensRequest{
			UserID:             user.ID,
			GoogleAccessToken:  token.AccessToken,
			GoogleRefreshToken: token.RefreshToken,
			GoogleTokenExpiry:  &token.Expiry,
			UpdateLastLogin:    true, // Update last login in same transaction
		}

		if err := h.userRepository.UpdateUserTokens(r.Context(), updateReq); err != nil {
			http.Error(w, "Failed to update user tokens", http.StatusInternalServerError)
			return
		}
	} else {
		// Create new user
		createReq := &database.CreateUserRequest{
			GoogleID:           userInfo.ID,
			Email:              userInfo.Email,
			Name:               userInfo.Name,
			ProfilePictureURL:  &userInfo.Picture,
			GoogleAccessToken:  token.AccessToken,
			GoogleRefreshToken: token.RefreshToken,
			GoogleTokenExpiry:  &token.Expiry,
		}

		user, err = h.userRepository.CreateUser(r.Context(), createReq)
		if err != nil {
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}
	}

	// Create session
	if err := h.createUserSession(w, r, user); err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Redirect to frontend dashboard
	dashboardURL := h.frontendURL + "/dashboard"
	http.Redirect(w, r, dashboardURL, http.StatusTemporaryRedirect)
}

// createUserSession creates a new session for the user and sets JWT cookie
func (h *AuthHandler) createUserSession(w http.ResponseWriter, r *http.Request, user *database.User) error {
	// Create session in database
	userAgent := r.Header.Get("User-Agent")
	ipAddress := middleware.GetClientIP(r)
	
	// Create session record first to get the actual session ID
	sessionReq := &database.CreateSessionRequest{
		UserID:    user.ID,
		UserAgent: &userAgent,
		IPAddress: &ipAddress,
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24 hour session
		// SessionToken will be set after generation
	}

	session, err := h.sessionRepository.CreateSession(r.Context(), sessionReq)
	if err != nil {
		return err
	}

	// Generate JWT token once with the actual session ID
	jwtToken, err := h.jwtService.GenerateToken(user.ID, user.Email, user.GoogleID, session.ID)
	if err != nil {
		return err
	}

	// Update session with the generated JWT token
	err = h.sessionRepository.UpdateSessionToken(r.Context(), session.ID, jwtToken)
	if err != nil {
		return err
	}

	// Set JWT as HttpOnly cookie
	domain, sameSite, secure := h.getCookieConfig()
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    jwtToken,
		Path:     "/",
		Domain:   domain,
		MaxAge:   int((24 * time.Hour).Seconds()), // 24 hours
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	}

	http.SetCookie(w, cookie)
	return nil
}

// GetCurrentUser returns the current authenticated user's information
func (h *AuthHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	user, err := h.userRepository.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}

	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Return public user data (no sensitive tokens)
	publicUser := user.ToPublicUser()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(publicUser); err != nil {
		http.Error(w, "Failed to encode user data", http.StatusInternalServerError)
		return
	}
}

// Logout handles user logout by invalidating the session
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := middleware.GetSessionIDFromContext(r.Context())
	if ok {
		// Deactivate session in database
		if err := h.sessionRepository.DeactivateSession(r.Context(), sessionID); err != nil {
			// Log error but don't fail the logout - user should still be logged out on client side
			// In production, this should use a proper logger
			// For now, we'll continue with clearing the cookie
		}
	}

	// Clear session cookie
	domain, sameSite, secure := h.getCookieConfig()
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

// RefreshToken refreshes the user's JWT token
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// Get current JWT token from cookie
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Error(w, "No session token", http.StatusUnauthorized)
		return
	}

	// Validate current token to get session ID
	claims, err := h.jwtService.ValidateToken(cookie.Value)
	if err != nil {
		http.Error(w, "Invalid session token", http.StatusUnauthorized)
		return
	}

	// Check if session is still active
	session, err := h.sessionRepository.GetSessionByID(r.Context(), claims.SessionID)
	if err != nil || session == nil || !session.IsActive {
		http.Error(w, "Session revoked or inactive", http.StatusUnauthorized)
		return
	}

	// Generate new token
	newToken, err := h.jwtService.RefreshToken(cookie.Value)
	if err != nil {
		http.Error(w, "Failed to refresh token", http.StatusUnauthorized)
		return
	}

	// Update session token in database
	if err := h.sessionRepository.UpdateSessionToken(r.Context(), claims.SessionID, newToken); err != nil {
		http.Error(w, "Failed to update session", http.StatusInternalServerError)
		return
	}

	// Set new JWT as HttpOnly cookie
	domain, sameSite, secure := h.getCookieConfig()
	newCookie := &http.Cookie{
		Name:     "session_token",
		Value:    newToken,
		Path:     "/",
		Domain:   domain,
		MaxAge:   int((24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	}

	http.SetCookie(w, newCookie)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Token refreshed successfully",
	}); err != nil {
		http.Error(w, "Failed to encode refresh response", http.StatusInternalServerError)
		return
	}
}


