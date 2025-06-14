package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
)

// AuthMiddleware provides authentication middleware for protected routes
type AuthMiddleware struct {
	jwtService        *auth.JWTService
	sessionRepository *database.SessionRepository
	oauthService      *auth.OAuthService
	userRepository    *database.UserRepository
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(jwtService *auth.JWTService, sessionRepository *database.SessionRepository, oauthService *auth.OAuthService, userRepository *database.UserRepository) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService:        jwtService,
		sessionRepository: sessionRepository,
		oauthService:      oauthService,
		userRepository:    userRepository,
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
		// Get JWT token from cookie
		cookie, err := r.Cookie("session_token")
		if err != nil {
			http.Error(w, "Unauthorized: No session token", http.StatusUnauthorized)
			return
		}

		// Validate JWT token
		claims, err := a.jwtService.ValidateToken(cookie.Value)
		if err != nil {
			http.Error(w, "Unauthorized: Invalid session token", http.StatusUnauthorized)
			return
		}

		// Verify session still exists and is active in database
		session, err := a.sessionRepository.GetSessionByToken(cookie.Value)
		if err != nil {
			http.Error(w, "Unauthorized: Session validation error", http.StatusUnauthorized)
			return
		}

		if session == nil {
			http.Error(w, "Unauthorized: Session not found or expired", http.StatusUnauthorized)
			return
		}

		// Update session last used timestamp
		if err := a.sessionRepository.UpdateSessionLastUsed(session.ID); err != nil {
			// Log error but don't fail the request
			// In production, you'd use a proper logger here
		}

		// Check and refresh OAuth tokens if necessary
		go a.checkAndRefreshOAuthTokens(r.Context(), claims.UserID)

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
		session, err := a.sessionRepository.GetSessionByToken(cookie.Value)
		if err != nil || session == nil {
			// Session not found or error, continue without authentication
			next.ServeHTTP(w, r)
			return
		}

		// Update session last used timestamp
		a.sessionRepository.UpdateSessionLastUsed(session.ID)

		// Add user information to request context
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, SessionIDKey, claims.SessionID)
		ctx = context.WithValue(ctx, EmailKey, claims.Email)

		// Continue to next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CORS middleware to handle cross-origin requests
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000") // Frontend URL
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

// GetClientIP extracts the client IP address from the request
func GetClientIP(r *http.Request) string {
	// Check for forwarded IP first (for load balancers/proxies)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
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
	user, err := a.userRepository.GetUserByID(userID)
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
	// Decrypt the refresh token
	refreshToken, err := a.userRepository.DecryptToken(user.GoogleRefreshToken)
	if err != nil {
		return // Failed to decrypt refresh token
	}

	// Refresh the token with Google
	newToken, err := a.oauthService.RefreshToken(ctx, refreshToken)
	if err != nil {
		return // Failed to refresh token
	}

	// Update the user's tokens in the database (background refresh, don't update last login)
	updateReq := &database.UpdateUserTokensRequest{
		UserID:             user.ID,
		GoogleAccessToken:  newToken.AccessToken,
		GoogleRefreshToken: newToken.RefreshToken,
		GoogleTokenExpiry:  &newToken.Expiry,
		UpdateLastLogin:    false, // Don't update last login for background token refresh
	}

	// Update in database (ignore errors since this is background operation)
	a.userRepository.UpdateUserTokens(updateReq)
}

// refreshStravaOAuthToken refreshes the user's Strava OAuth token
// Note: This would require a Strava OAuth service to be implemented
func (a *AuthMiddleware) refreshStravaOAuthToken(ctx context.Context, user *database.User) {
	// TODO: Implement Strava token refresh when Strava OAuth service is available
	// This is a placeholder for future Strava integration
}