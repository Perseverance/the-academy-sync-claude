package auth

import (
	"context"
	"time"
)

// TokenPersister defines the interface for persisting OAuth tokens
// This interface is used by API clients to save refreshed tokens back to the database
type TokenPersister interface {
	// UpdateStravaTokens updates Strava OAuth tokens for a user
	UpdateStravaTokens(ctx context.Context, userID int, accessToken, refreshToken string, expiry time.Time) error

	// UpdateGoogleTokens updates Google OAuth tokens for a user
	UpdateGoogleTokens(ctx context.Context, userID int, accessToken, refreshToken string, expiry time.Time) error
}
