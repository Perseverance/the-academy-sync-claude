package middleware

import (
	"net/http"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// PlaceholderAuth provides placeholder authentication middleware for scheduler endpoints
// TODO: Implement proper OIDC authentication (TECH-011) before production deployment
type PlaceholderAuth struct {
	logger *logger.Logger
}

// NewPlaceholderAuth creates a new placeholder authentication middleware
func NewPlaceholderAuth(logger *logger.Logger) *PlaceholderAuth {
	return &PlaceholderAuth{
		logger: logger,
	}
}

// Authenticate middleware that performs placeholder authentication
// Currently always allows requests through but is prepared to return 401 when OIDC is implemented
func (p *PlaceholderAuth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.logger.Debug("Placeholder auth middleware processing request",
			"path", r.URL.Path,
			"method", r.Method,
			"client_ip", GetClientIP(r),
			"user_agent", r.Header.Get("User-Agent"))

		// TODO (TECH-011): Implement proper OIDC token validation
		// For now, this is a placeholder that always allows requests through
		// 
		// Future implementation should:
		// 1. Extract Bearer token from Authorization header
		// 2. Validate OIDC token signature and claims
		// 3. Verify the service account email matches allowed accounts
		// 4. Return 401 Unauthorized if validation fails
		//
		// Example of future validation (currently commented out):
		/*
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			p.logger.Warn("Missing Authorization header")
			http.Error(w, `{"error": "unauthorized", "message": "Missing Authorization header"}`, http.StatusUnauthorized)
			return
		}

		// Extract bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			p.logger.Warn("Invalid Authorization header format")
			http.Error(w, `{"error": "unauthorized", "message": "Invalid Authorization header format"}`, http.StatusUnauthorized)
			return
		}

		token := parts[1]
		
		// TODO: Validate OIDC token here
		if !p.validateOIDCToken(token) {
			p.logger.Warn("Invalid OIDC token")
			http.Error(w, `{"error": "unauthorized", "message": "Invalid OIDC token"}`, http.StatusUnauthorized)
			return
		}
		*/

		// For now, always allow the request through
		p.logger.Info("Placeholder auth: allowing request (OIDC not yet implemented)")
		
		// Call the next handler
		next.ServeHTTP(w, r)
	})
}