package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// StravaHandler handles Strava OAuth-related HTTP requests
type StravaHandler struct {
	oauthService      *auth.OAuthService
	userRepository    *database.UserRepository
	frontendURL       string
	isDevelopment     bool
	logger            *logger.Logger
}

// NewStravaHandler creates a new Strava handler
func NewStravaHandler(
	oauthService *auth.OAuthService,
	userRepository *database.UserRepository,
	frontendURL string,
	isDevelopment bool,
	logger *logger.Logger,
) *StravaHandler {
	return &StravaHandler{
		oauthService:   oauthService,
		userRepository: userRepository,
		frontendURL:    frontendURL,
		isDevelopment:  isDevelopment,
		logger:         logger,
	}
}

// getCookieConfig returns appropriate cookie configuration for the environment
func (h *StravaHandler) getCookieConfig() (domain string, sameSite http.SameSite, secure bool) {
	if h.isDevelopment {
		// Development: Use .localhost domain to share cookies across ports
		// SameSite=Lax allows cookies in top-level navigation (OAuth redirects)
		return ".localhost", http.SameSiteLaxMode, false
	}
	// Production: Use secure settings
	return "", http.SameSiteLaxMode, true
}

// generateSecureStravaState generates a cryptographically secure random state for OAuth CSRF protection
// that includes the user ID for session correlation
func generateSecureStravaState(userID int) (string, error) {
	// Generate 16 bytes (128 bits) of cryptographically secure random data
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate secure random state: %w", err)
	}
	
	// Create state with format: "strava-{userID}-{randomBytes}"
	// This allows us to correlate the OAuth callback with the user session
	randomString := base64.URLEncoding.EncodeToString(randomBytes)
	return fmt.Sprintf("strava-%d-%s", userID, randomString), nil
}

// parseUserIDFromStravaState extracts the user ID from the Strava OAuth state parameter
func parseUserIDFromStravaState(state string) (int, error) {
	// Expected format: "strava-{userID}-{randomBytes}"
	parts := strings.Split(state, "-")
	if len(parts) < 3 || parts[0] != "strava" {
		return 0, fmt.Errorf("invalid state format: expected 'strava-{userID}-{random}', got '%s'", state)
	}
	
	userID, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid user ID in state parameter: %w", err)
	}
	
	return userID, nil
}

// StravaAuthURL generates and returns the Strava OAuth authorization URL
func (h *StravaHandler) StravaAuthURL(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	clientIP := middleware.GetClientIP(r)
	
	h.logger.Debug("Generating Strava OAuth authorization URL", 
		"user_id", userID,
		"has_user_id", ok,
		"client_ip", clientIP,
		"user_agent", r.Header.Get("User-Agent"))
	
	if !ok {
		h.logger.Warn("StravaAuthURL called without valid user context", 
			"client_ip", clientIP)
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}
	
	// Generate a cryptographically secure state parameter for CSRF protection
	// Include user ID in state for session correlation on callback
	state, err := generateSecureStravaState(userID)
	if err != nil {
		h.logger.Error("Failed to generate secure Strava OAuth state", "error", err, "user_id", userID)
		http.Error(w, "Failed to generate secure state", http.StatusInternalServerError)
		return
	}
	
	// Note: We don't store state in cookies since Strava redirects directly to backend
	// The user ID is embedded in the state parameter for session correlation

	authURL := h.oauthService.GetStravaAuthURL(state)
	
	h.logger.Debug("Generated Strava OAuth URL", 
		"user_id", userID,
		"state_length", len(state),
		"auth_url_length", len(authURL))

	response := map[string]string{
		"auth_url": authURL,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode Strava OAuth URL response", "error", err, "user_id", userID)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	
	h.logger.Info("Strava OAuth URL generated successfully", "user_id", userID)
}

// StravaCallback handles the OAuth callback from Strava
func (h *StravaHandler) StravaCallback(w http.ResponseWriter, r *http.Request) {
	clientIP := middleware.GetClientIP(r)
	
	h.logger.Debug("Handling Strava OAuth callback", 
		"client_ip", clientIP,
		"user_agent", r.Header.Get("User-Agent"))
	
	// Get state parameter and extract user ID from it
	stateParam := r.URL.Query().Get("state")
	if stateParam == "" {
		h.logger.Warn("Strava OAuth callback missing state parameter", "client_ip", clientIP)
		http.Error(w, "Missing state parameter", http.StatusBadRequest)
		return
	}
	
	// Extract user ID from state parameter
	userID, err := parseUserIDFromStravaState(stateParam)
	if err != nil {
		h.logger.Error("Failed to parse user ID from Strava OAuth state", 
			"error", err, 
			"state", stateParam,
			"client_ip", clientIP)
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}
	
	h.logger.Debug("Extracted user ID from Strava OAuth state", 
		"user_id", userID,
		"state_length", len(stateParam))
	
	// Check for error parameter (user denied access)
	if errorParam := r.URL.Query().Get("error"); errorParam != "" {
		h.logger.Info("User denied Strava OAuth access", 
			"user_id", userID,
			"error", errorParam,
			"client_ip", clientIP)
		
		// Redirect to dashboard (frontend can handle error states through UI)
		dashboardURL := h.frontendURL + "/dashboard"
		h.logger.Debug("Redirecting to dashboard after Strava access denied", 
			"user_id", userID,
			"redirect_url", dashboardURL)
		http.Redirect(w, r, dashboardURL, http.StatusTemporaryRedirect)
		return
	}

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		h.logger.Warn("Strava OAuth callback missing authorization code", 
			"user_id", userID,
			"client_ip", clientIP)
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}
	
	h.logger.Debug("Exchanging Strava OAuth authorization code for token", 
		"user_id", userID,
		"code_length", len(code))

	// Exchange code for token
	token, err := h.oauthService.ExchangeStravaCodeForToken(context.Background(), code)
	if err != nil {
		h.logger.Error("Failed to exchange Strava OAuth code for token", 
			"error", err, 
			"user_id", userID,
			"client_ip", clientIP)
		http.Error(w, "Failed to exchange code for token", http.StatusInternalServerError)
		return
	}
	
	h.logger.Debug("Successfully exchanged Strava OAuth code for token", 
		"user_id", userID,
		"token_type", token.TokenType, 
		"has_access_token", len(token.AccessToken) > 0,
		"has_refresh_token", len(token.RefreshToken) > 0,
		"token_expires_at", token.Expiry.Format("2006-01-02 15:04:05"))

	// Get athlete info from Strava
	athleteInfo, err := h.oauthService.GetStravaUserInfo(context.Background(), token)
	if err != nil {
		h.logger.Error("Failed to get athlete info from Strava", 
			"error", err, 
			"user_id", userID,
			"client_ip", clientIP)
		http.Error(w, "Failed to get athlete info", http.StatusInternalServerError)
		return
	}
	
	h.logger.Debug("Retrieved athlete info from Strava", 
		"user_id", userID,
		"athlete_id", athleteInfo.ID,
		"athlete_name", fmt.Sprintf("%s %s", athleteInfo.FirstName, athleteInfo.LastName),
		"athlete_city", athleteInfo.City,
		"athlete_country", athleteInfo.Country)

	// Update user's Strava connection in database
	h.logger.Debug("Updating user's Strava connection in database", 
		"user_id", userID,
		"athlete_id", athleteInfo.ID)
	
	// Construct full athlete name
	athleteName := fmt.Sprintf("%s %s", athleteInfo.FirstName, athleteInfo.LastName)
	
	if err := h.userRepository.UpdateStravaConnection(r.Context(), userID, token.AccessToken, token.RefreshToken, &token.Expiry, athleteInfo.ID, athleteName, athleteInfo.Profile); err != nil {
		h.logger.Error("Failed to update user's Strava connection", 
			"error", err, 
			"user_id", userID,
			"athlete_id", athleteInfo.ID)
		http.Error(w, "Failed to save Strava connection", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully saved Strava connection to database", 
		"user_id", userID,
		"athlete_id", athleteInfo.ID)

	// Check if all prerequisites are now met and enable automation if ready
	if err := h.userRepository.CheckAndEnableAutomationIfReady(r.Context(), userID); err != nil {
		h.logger.Warn("Failed to check/enable automation after Strava connection",
			"error", err,
			"user_id", userID)
		// Don't fail the Strava connection for this error
	}

	// Redirect to frontend dashboard (clean URL - frontend will detect connection automatically)
	dashboardURL := h.frontendURL + "/dashboard"
	h.logger.Info("Strava OAuth callback successful, redirecting to dashboard", 
		"user_id", userID,
		"athlete_id", athleteInfo.ID,
		"athlete_name", fmt.Sprintf("%s %s", athleteInfo.FirstName, athleteInfo.LastName),
		"dashboard_url", dashboardURL,
		"client_ip", clientIP)
	http.Redirect(w, r, dashboardURL, http.StatusTemporaryRedirect)
}

// DisconnectStrava handles disconnecting the user's Strava account
func (h *StravaHandler) DisconnectStrava(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	clientIP := middleware.GetClientIP(r)
	
	h.logger.Debug("Disconnecting Strava account", 
		"user_id", userID,
		"has_user_id", ok,
		"client_ip", clientIP)
	
	if !ok {
		h.logger.Warn("DisconnectStrava called without valid user context", 
			"client_ip", clientIP)
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Remove Strava connection from database
	h.logger.Debug("Removing Strava connection from database", "user_id", userID)
	
	if err := h.userRepository.RemoveStravaConnection(r.Context(), userID); err != nil {
		h.logger.Error("Failed to remove user's Strava connection", 
			"error", err, 
			"user_id", userID,
			"client_ip", clientIP)
		http.Error(w, "Failed to disconnect Strava account", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully disconnected Strava account", 
		"user_id", userID,
		"client_ip", clientIP)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Strava account disconnected successfully",
	}); err != nil {
		h.logger.Error("Failed to encode disconnect response", 
			"error", err, 
			"user_id", userID)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}