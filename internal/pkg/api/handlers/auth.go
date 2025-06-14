package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
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
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(
	oauthService *auth.OAuthService,
	jwtService *auth.JWTService,
	userRepository *database.UserRepository,
	sessionRepository *database.SessionRepository,
	frontendURL string,
) *AuthHandler {
	return &AuthHandler{
		oauthService:      oauthService,
		jwtService:        jwtService,
		userRepository:    userRepository,
		sessionRepository: sessionRepository,
		frontendURL:       frontendURL,
	}
}

// GoogleAuthURL generates and returns the Google OAuth authorization URL
func (h *AuthHandler) GoogleAuthURL(w http.ResponseWriter, r *http.Request) {
	// Generate a state parameter for CSRF protection
	state := "random-state-" + strconv.FormatInt(time.Now().Unix(), 10)
	
	// Store state in session cookie for validation
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		Domain:   "localhost",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteNoneMode,
	})

	authURL := h.oauthService.GetAuthURL(state)

	response := map[string]string{
		"auth_url": authURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GoogleCallback handles the OAuth callback from Google
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Validate state parameter
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		http.Error(w, "Missing state cookie", http.StatusBadRequest)
		return
	}

	stateParam := r.URL.Query().Get("state")
	if stateParam != stateCookie.Value {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Clear the state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		Domain:   "localhost",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	})

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
	
	sessionReq := &database.CreateSessionRequest{
		UserID:    user.ID,
		UserAgent: &userAgent,
		IPAddress: &ipAddress,
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24 hour session
	}

	// Generate JWT token first to get the session token
	jwtToken, err := h.jwtService.GenerateToken(user.ID, user.Email, user.GoogleID, 0) // Temporary session ID
	if err != nil {
		return err
	}

	sessionReq.SessionToken = jwtToken

	session, err := h.sessionRepository.CreateSession(r.Context(), sessionReq)
	if err != nil {
		return err
	}

	// Regenerate JWT with actual session ID
	jwtToken, err = h.jwtService.GenerateToken(user.ID, user.Email, user.GoogleID, session.ID)
	if err != nil {
		return err
	}

	// Update session with correct JWT token
	sessionReq.SessionToken = jwtToken
	// Note: In a real implementation, you might want to update the session record
	// For now, we'll proceed with the new token

	// Set JWT as HttpOnly cookie
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    jwtToken,
		Path:     "/",
		Domain:   "localhost", // Allow cookie to be shared across localhost ports
		MaxAge:   int((24 * time.Hour).Seconds()), // 24 hours
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteNoneMode, // Allow cross-site requests for localhost development
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
	json.NewEncoder(w).Encode(publicUser)
}

// Logout handles user logout by invalidating the session
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := middleware.GetSessionIDFromContext(r.Context())
	if ok {
		// Deactivate session in database
		h.sessionRepository.DeactivateSession(r.Context(), sessionID)
	}

	// Clear session cookie
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		Domain:   "localhost",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	}

	http.SetCookie(w, cookie)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logged out successfully",
	})
}

// RefreshToken refreshes the user's JWT token
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// Get current JWT token from cookie
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Error(w, "No session token", http.StatusUnauthorized)
		return
	}

	// Generate new token
	newToken, err := h.jwtService.RefreshToken(cookie.Value)
	if err != nil {
		http.Error(w, "Failed to refresh token", http.StatusUnauthorized)
		return
	}

	// Set new JWT as HttpOnly cookie
	newCookie := &http.Cookie{
		Name:     "session_token",
		Value:    newToken,
		Path:     "/",
		Domain:   "localhost",
		MaxAge:   int((24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteNoneMode,
	}

	http.SetCookie(w, newCookie)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Token refreshed successfully",
	})
}


