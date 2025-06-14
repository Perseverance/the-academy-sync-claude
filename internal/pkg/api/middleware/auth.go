package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// AuthMiddleware provides authentication middleware for protected routes
type AuthMiddleware struct {
	jwtService        *auth.JWTService
	sessionRepository *database.SessionRepository
	oauthService      *auth.OAuthService
	userRepository    *database.UserRepository
	logger            *logger.Logger
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(jwtService *auth.JWTService, sessionRepository *database.SessionRepository, oauthService *auth.OAuthService, userRepository *database.UserRepository, logger *logger.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService:        jwtService,
		sessionRepository: sessionRepository,
		oauthService:      oauthService,
		userRepository:    userRepository,
		logger:            logger,
	}
}

// ContextKey is used for storing values in request context
type ContextKey string

const (
	// UserIDKey is the context key for user ID
	UserIDKey ContextKey = "user_id"
	// SessionIDKey is the context key for session ID
	SessionIDKey ContextKey = "session_id"
	// EmailKey is the context key for user email
	EmailKey ContextKey = "email"
)

// RequireAuth middleware validates JWT tokens and ensures user is authenticated
func (a *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := GetClientIP(r)
		
		a.logger.Debug("Auth middleware processing request",
			"path", r.URL.Path,
			"method", r.Method,
			"client_ip", clientIP,
			"user_agent", r.Header.Get("User-Agent"))
		
		// Get JWT token from cookie
		cookie, err := r.Cookie("session_token")
		if err != nil {
			a.logger.Warn("Authentication failed: No session token cookie",
				"path", r.URL.Path,
				"client_ip", clientIP,
				"cookie_error", err.Error())
			http.Error(w, "Unauthorized: No session token", http.StatusUnauthorized)
			return
		}

		// Validate JWT token
		claims, err := a.jwtService.ValidateToken(cookie.Value)
		if err != nil {
			a.logger.Warn("Authentication failed: Invalid JWT token",
				"path", r.URL.Path,
				"client_ip", clientIP,
				"validation_error", err.Error())
			http.Error(w, "Unauthorized: Invalid session token", http.StatusUnauthorized)
			return
		}

		// Verify session still exists and is active in database
		session, err := a.sessionRepository.GetSessionByToken(r.Context(), cookie.Value)
		if err != nil {
			a.logger.Error("Authentication failed: Session validation database error",
				"path", r.URL.Path,
				"client_ip", clientIP,
				"user_id", claims.UserID,
				"session_id", claims.SessionID,
				"error", err.Error())
			http.Error(w, "Unauthorized: Session validation error", http.StatusUnauthorized)
			return
		}

		if session == nil {
			a.logger.Warn("Authentication failed: Session not found or expired",
				"path", r.URL.Path,
				"client_ip", clientIP,
				"user_id", claims.UserID,
				"session_id", claims.SessionID)
			http.Error(w, "Unauthorized: Session not found or expired", http.StatusUnauthorized)
			return
		}

		// Update session last used timestamp
		if err := a.sessionRepository.UpdateSessionLastUsed(r.Context(), session.ID); err != nil {
			a.logger.Error("Failed to update session last used timestamp",
				"session_id", session.ID,
				"user_id", claims.UserID,
				"error", err.Error())
			// Don't fail the request - this is a non-critical operation
		}

		// Check and refresh OAuth tokens if necessary
		go a.checkAndRefreshOAuthTokens(context.Background(), claims.UserID)

		a.logger.Debug("Authentication successful",
			"path", r.URL.Path,
			"user_id", claims.UserID,
			"session_id", claims.SessionID,
			"client_ip", clientIP)

		// Add user information to request context
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, SessionIDKey, claims.SessionID)
		ctx = context.WithValue(ctx, EmailKey, claims.Email)

		// Continue to next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserIDFromContext extracts the user ID from the request context
func GetUserIDFromContext(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(UserIDKey).(int)
	return userID, ok
}

// GetSessionIDFromContext extracts the session ID from the request context
func GetSessionIDFromContext(ctx context.Context) (int, bool) {
	sessionID, ok := ctx.Value(SessionIDKey).(int)
	return sessionID, ok
}

// GetEmailFromContext extracts the user email from the request context
func GetEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(EmailKey).(string)
	return email, ok
}

// OptionalAuth middleware validates JWT tokens but doesn't require authentication
// Useful for endpoints that behave differently for authenticated vs anonymous users
func (a *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to get JWT token from cookie
		cookie, err := r.Cookie("session_token")
		if err != nil {
			// No token present, continue without authentication
			next.ServeHTTP(w, r)
			return
		}

		// Try to validate JWT token
		claims, err := a.jwtService.ValidateToken(cookie.Value)
		if err != nil {
			// Invalid token, continue without authentication
			next.ServeHTTP(w, r)
			return
		}

		// Try to verify session exists and is active
		session, err := a.sessionRepository.GetSessionByToken(r.Context(), cookie.Value)
		if err != nil || session == nil {
			// Session not found or error, continue without authentication
			next.ServeHTTP(w, r)
			return
		}

		// Update session last used timestamp
		if err := a.sessionRepository.UpdateSessionLastUsed(r.Context(), session.ID); err != nil {
			// Log error but don't fail the optional auth request
			// In production, this should use a structured logger
		}

		// Add user information to request context
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, SessionIDKey, claims.SessionID)
		ctx = context.WithValue(ctx, EmailKey, claims.Email)

		// Continue to next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CORS middleware to handle cross-origin requests
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetClientIP extracts the client IP address from the request
func GetClientIP(r *http.Request) string {
	// Check for forwarded IP first (for load balancers/proxies)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For may contain comma-separated list: "client, proxy1, proxy2"
		// The first IP is the original client IP
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Strip port from RemoteAddr (format is "IP:port" or "[IPv6]:port")
	addr := r.RemoteAddr
	if addr != "" {
		// Handle IPv6 addresses with brackets
		if addr[0] == '[' {
			if idx := strings.Index(addr, "]:"); idx != -1 {
				return addr[1:idx] // Return IPv6 without brackets and port
			}
		} else {
			// Handle IPv4 addresses
			if idx := strings.LastIndex(addr, ":"); idx != -1 {
				return addr[:idx] // Return IP without port
			}
		}
	}
	
	return addr
}

// checkAndRefreshOAuthTokens checks if the user's OAuth tokens need refreshing and updates them
// This runs asynchronously to avoid blocking the request
func (a *AuthMiddleware) checkAndRefreshOAuthTokens(ctx context.Context, userID int) {
	if a.oauthService == nil || a.userRepository == nil {
		return // OAuth services not available
	}

	// Get user from database to check token expiry
	user, err := a.userRepository.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return // User not found or error
	}

	// Check if Google OAuth token needs refreshing (refresh if expires within 5 minutes)
	if a.shouldRefreshGoogleToken(user) {
		a.refreshGoogleOAuthToken(ctx, user)
	}

	// Check if Strava OAuth token needs refreshing (refresh if expires within 5 minutes)
	if a.shouldRefreshStravaToken(user) {
		a.refreshStravaOAuthToken(ctx, user)
	}
}

// shouldRefreshGoogleToken checks if the Google OAuth token needs refreshing
func (a *AuthMiddleware) shouldRefreshGoogleToken(user *database.User) bool {
	if user.GoogleTokenExpiry == nil {
		return false // No expiry set, assume token is still valid
	}

	// Refresh if token expires within 5 minutes
	return time.Until(*user.GoogleTokenExpiry) < 5*time.Minute
}

// shouldRefreshStravaToken checks if the Strava OAuth token needs refreshing
func (a *AuthMiddleware) shouldRefreshStravaToken(user *database.User) bool {
	if user.StravaTokenExpiry == nil {
		return false // No expiry set, assume token is still valid
	}

	// Refresh if token expires within 5 minutes
	return time.Until(*user.StravaTokenExpiry) < 5*time.Minute
}

// refreshGoogleOAuthToken refreshes the user's Google OAuth token
func (a *AuthMiddleware) refreshGoogleOAuthToken(ctx context.Context, user *database.User) {
	a.logger.Debug("Starting background Google OAuth token refresh", "user_id", user.ID)
	
	// Decrypt the refresh token
	refreshToken, err := a.userRepository.DecryptToken(user.GoogleRefreshToken)
	if err != nil {
		a.logger.Error("Failed to decrypt Google refresh token for background refresh",
			"user_id", user.ID, "error", err.Error())
		return
	}

	// Refresh the token with Google
	newToken, err := a.oauthService.RefreshToken(ctx, refreshToken)
	if err != nil {
		a.logger.Error("Failed to refresh Google OAuth token",
			"user_id", user.ID, "error", err.Error())
		return
	}

	// Update the user's tokens in the database (background refresh, don't update last login)
	updateReq := &database.UpdateUserTokensRequest{
		UserID:             user.ID,
		GoogleAccessToken:  newToken.AccessToken,
		GoogleRefreshToken: newToken.RefreshToken,
		GoogleTokenExpiry:  &newToken.Expiry,
		UpdateLastLogin:    false, // Don't update last login for background token refresh
	}

	// Update in database
	if err := a.userRepository.UpdateUserTokens(ctx, updateReq); err != nil {
		a.logger.Error("Failed to update user tokens after Google OAuth refresh",
			"user_id", user.ID, "error", err.Error())
		return
	}
	
	a.logger.Info("Successfully refreshed Google OAuth token in background",
		"user_id", user.ID, "new_expiry", newToken.Expiry.String())
}

// refreshStravaOAuthToken refreshes the user's Strava OAuth token
// Note: This would require a Strava OAuth service to be implemented
func (a *AuthMiddleware) refreshStravaOAuthToken(ctx context.Context, user *database.User) {
	// TODO: Implement Strava token refresh when Strava OAuth service is available
	// This is a placeholder for future Strava integration
}