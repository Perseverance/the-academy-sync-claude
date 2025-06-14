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
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	oauthService      *auth.OAuthService
	jwtService        *auth.JWTService
	userRepository    *database.UserRepository
	sessionRepository *database.SessionRepository
	frontendURL       string
	isDevelopment     bool
	logger            *logger.Logger
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(
	oauthService *auth.OAuthService,
	jwtService *auth.JWTService,
	userRepository *database.UserRepository,
	sessionRepository *database.SessionRepository,
	frontendURL string,
	isDevelopment bool,
	logger *logger.Logger,
) *AuthHandler {
	return &AuthHandler{
		oauthService:      oauthService,
		jwtService:        jwtService,
		userRepository:    userRepository,
		sessionRepository: sessionRepository,
		frontendURL:       frontendURL,
		isDevelopment:     isDevelopment,
		logger:            logger,
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
	h.logger.Debug("Generating Google OAuth authorization URL", 
		"client_ip", middleware.GetClientIP(r),
		"user_agent", r.Header.Get("User-Agent"))
	
	// Generate a cryptographically secure state parameter for CSRF protection
	state, err := generateSecureState()
	if err != nil {
		h.logger.Error("Failed to generate secure OAuth state", "error", err)
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
	
	h.logger.Debug("Generated Google OAuth URL", 
		"state_length", len(state),
		"cookie_domain", domain,
		"cookie_secure", secure)

	response := map[string]string{
		"auth_url": authURL,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode OAuth URL response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	
	h.logger.Info("Google OAuth URL generated successfully")
}

// GoogleCallback handles the OAuth callback from Google
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling Google OAuth callback", 
		"client_ip", middleware.GetClientIP(r),
		"user_agent", r.Header.Get("User-Agent"))
	
	// Validate state parameter exists
	stateParam := r.URL.Query().Get("state")
	if stateParam == "" {
		h.logger.Warn("OAuth callback missing state parameter", "client_ip", middleware.GetClientIP(r))
		http.Error(w, "Missing state parameter", http.StatusBadRequest)
		return
	}

	// Validate state cookie exists and matches (required for security)
	stateCookie, cookieErr := r.Cookie("oauth_state")
	if cookieErr != nil {
		h.logger.Debug("OAuth state cookie not found", "cookie_error", cookieErr.Error(), "is_development", h.isDevelopment)
		// In production, state cookie is required for CSRF protection
		if !h.isDevelopment {
			h.logger.Warn("OAuth callback missing state cookie in production", "client_ip", middleware.GetClientIP(r))
			http.Error(w, "Missing state cookie - CSRF protection required", http.StatusBadRequest)
			return
		}
		// In development, allow fallback validation for direct callback testing
		if !strings.HasPrefix(stateParam, "oauth-") {
			h.logger.Warn("OAuth callback invalid state parameter format in development", 
				"state_length", len(stateParam),
				"has_oauth_prefix", false)
			http.Error(w, "Invalid state parameter format", http.StatusBadRequest)
			return
		}
		h.logger.Debug("Using fallback state validation in development")
	} else {
		// Cookie exists, must match exactly
		if stateCookie.Value != stateParam {
			h.logger.Warn("OAuth state parameter mismatch - CSRF protection failed", 
				"client_ip", middleware.GetClientIP(r),
				"cookie_state_length", len(stateCookie.Value),
				"param_state_length", len(stateParam),
				"states_match", false)
			http.Error(w, "Invalid state parameter - CSRF protection failed", http.StatusBadRequest)
			return
		}
		h.logger.Debug("OAuth state validation successful")
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
		h.logger.Warn("OAuth callback missing authorization code", "client_ip", middleware.GetClientIP(r))
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}
	
	h.logger.Debug("Exchanging OAuth authorization code for token", "code_length", len(code))

	// Exchange code for token
	token, err := h.oauthService.ExchangeCodeForToken(context.Background(), code)
	if err != nil {
		h.logger.Error("Failed to exchange OAuth code for token", "error", err, "client_ip", middleware.GetClientIP(r))
		http.Error(w, "Failed to exchange code for token", http.StatusInternalServerError)
		return
	}
	
	h.logger.Debug("Successfully exchanged OAuth code for token", "token_type", token.TokenType, "expires_in", token.Expiry.Sub(time.Now()).String())

	// Get user info from Google
	userInfo, err := h.oauthService.GetUserInfo(context.Background(), token)
	if err != nil {
		h.logger.Error("Failed to get user info from Google", "error", err, "client_ip", middleware.GetClientIP(r))
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}
	
	h.logger.Debug("Retrieved user info from Google", "user_id", userInfo.ID, "email", userInfo.Email, "name", userInfo.Name)

	// Check if user already exists
	existingUser, err := h.userRepository.GetUserByGoogleID(r.Context(), userInfo.ID)
	if err != nil {
		h.logger.Error("Database error while checking existing user", "error", err, "google_user_id", userInfo.ID)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var user *database.User

	if existingUser != nil {
		h.logger.Info("Existing user login", "user_id", existingUser.ID, "email", existingUser.Email)
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
			h.logger.Error("Failed to update existing user tokens", "error", err, "user_id", user.ID)
			http.Error(w, "Failed to update user tokens", http.StatusInternalServerError)
			return
		}
		h.logger.Debug("Updated existing user tokens successfully", "user_id", user.ID)
	} else {
		h.logger.Info("Creating new user", "google_user_id", userInfo.ID, "email", userInfo.Email)
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
			h.logger.Error("Failed to create new user", "error", err, "google_user_id", userInfo.ID, "email", userInfo.Email)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}
		h.logger.Info("Created new user successfully", "user_id", user.ID, "email", user.Email)
	}

	// Create session
	if err := h.createUserSession(w, r, user); err != nil {
		h.logger.Error("Failed to create user session", "error", err, "user_id", user.ID)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Redirect to frontend dashboard
	dashboardURL := h.frontendURL + "/dashboard"
	h.logger.Info("OAuth callback successful, redirecting to dashboard", 
		"user_id", user.ID, 
		"email", user.Email,
		"dashboard_url", dashboardURL,
		"client_ip", middleware.GetClientIP(r))
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
	sessionID, hasSession := middleware.GetSessionIDFromContext(r.Context())
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	
	h.logger.Debug("GetCurrentUser API request", 
		"user_id", userID,
		"has_user_id", ok,
		"session_id", sessionID,
		"has_session", hasSession,
		"client_ip", middleware.GetClientIP(r),
		"user_agent", r.Header.Get("User-Agent"))
	
	if !ok {
		h.logger.Warn("GetCurrentUser called without valid user context", 
			"client_ip", middleware.GetClientIP(r))
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	user, err := h.userRepository.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to fetch user from database", 
			"error", err, 
			"user_id", userID,
			"client_ip", middleware.GetClientIP(r))
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}

	if user == nil {
		h.logger.Warn("User not found in database", 
			"user_id", userID,
			"client_ip", middleware.GetClientIP(r))
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Return public user data (no sensitive tokens)
	publicUser := user.ToPublicUser()
	
	h.logger.Debug("Returning user information", 
		"user_id", user.ID,
		"email", user.Email,
		"name", user.Name,
		"has_strava_connection", publicUser.HasStravaConnection,
		"has_sheets_connection", publicUser.HasSheetsConnection,
		"timezone", user.Timezone)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(publicUser); err != nil {
		h.logger.Error("Failed to encode user data response", 
			"error", err, 
			"user_id", user.ID,
			"client_ip", middleware.GetClientIP(r))
		http.Error(w, "Failed to encode user data", http.StatusInternalServerError)
		return
	}
	
	h.logger.Info("GetCurrentUser request completed successfully", 
		"user_id", user.ID,
		"email", user.Email)
}

// Logout handles user logout by invalidating the session
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	userID, hasUserID := middleware.GetUserIDFromContext(r.Context())
	sessionID, ok := middleware.GetSessionIDFromContext(r.Context())
	
	h.logger.Info("User logout initiated", 
		"user_id", userID, 
		"has_user_id", hasUserID,
		"session_id", sessionID,
		"has_session", ok,
		"client_ip", middleware.GetClientIP(r))
	
	if ok {
		// Deactivate session in database
		if err := h.sessionRepository.DeactivateSession(r.Context(), sessionID); err != nil {
			h.logger.Error("Failed to deactivate session during logout", 
				"error", err, 
				"session_id", sessionID,
				"user_id", userID)
			// Continue with clearing the cookie anyway
		} else {
			h.logger.Debug("Successfully deactivated session", "session_id", sessionID)
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
	h.logger.Debug("RefreshToken request initiated", 
		"client_ip", middleware.GetClientIP(r),
		"user_agent", r.Header.Get("User-Agent"))
	
	// Get current JWT token from cookie
	cookie, err := r.Cookie("session_token")
	if err != nil {
		h.logger.Warn("RefreshToken request missing session token cookie", 
			"cookie_error", err.Error(),
			"client_ip", middleware.GetClientIP(r))
		http.Error(w, "No session token", http.StatusUnauthorized)
		return
	}

	// Validate current token to get session ID
	claims, err := h.jwtService.ValidateToken(cookie.Value)
	if err != nil {
		h.logger.Warn("RefreshToken request with invalid JWT token", 
			"validation_error", err.Error(),
			"client_ip", middleware.GetClientIP(r))
		http.Error(w, "Invalid session token", http.StatusUnauthorized)
		return
	}
	
	h.logger.Debug("JWT token validated successfully", 
		"user_id", claims.UserID,
		"session_id", claims.SessionID,
		"email", claims.Email)

	// Check if session is still active
	session, err := h.sessionRepository.GetSessionByID(r.Context(), claims.SessionID)
	if err != nil || session == nil || !session.IsActive {
		h.logger.Warn("RefreshToken request for inactive or revoked session", 
			"session_id", claims.SessionID,
			"user_id", claims.UserID,
			"session_found", session != nil,
			"session_active", session != nil && session.IsActive,
			"db_error", err,
			"client_ip", middleware.GetClientIP(r))
		http.Error(w, "Session revoked or inactive", http.StatusUnauthorized)
		return
	}

	// Generate new token
	newToken, err := h.jwtService.RefreshToken(cookie.Value)
	if err != nil {
		h.logger.Error("Failed to generate new JWT token during refresh", 
			"error", err,
			"user_id", claims.UserID,
			"session_id", claims.SessionID,
			"client_ip", middleware.GetClientIP(r))
		http.Error(w, "Failed to refresh token", http.StatusUnauthorized)
		return
	}
	
	h.logger.Debug("Generated new JWT token", 
		"user_id", claims.UserID,
		"session_id", claims.SessionID)

	// Update session token in database
	if err := h.sessionRepository.UpdateSessionToken(r.Context(), claims.SessionID, newToken); err != nil {
		h.logger.Error("Failed to update session token in database", 
			"error", err,
			"user_id", claims.UserID,
			"session_id", claims.SessionID,
			"client_ip", middleware.GetClientIP(r))
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
	
	h.logger.Debug("Set new JWT cookie", 
		"cookie_domain", domain,
		"cookie_secure", secure,
		"user_id", claims.UserID,
		"session_id", claims.SessionID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Token refreshed successfully",
	}); err != nil {
		h.logger.Error("Failed to encode refresh token response", 
			"error", err,
			"user_id", claims.UserID,
			"client_ip", middleware.GetClientIP(r))
		http.Error(w, "Failed to encode refresh response", http.StatusInternalServerError)
		return
	}
	
	h.logger.Info("Token refresh completed successfully", 
		"user_id", claims.UserID,
		"email", claims.Email,
		"session_id", claims.SessionID)
}


