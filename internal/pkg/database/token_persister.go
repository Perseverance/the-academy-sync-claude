package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// Ensure TokenPersister implements the auth.TokenPersister interface
var _ auth.TokenPersister = (*TokenPersister)(nil)

// TokenPersister provides database persistence for OAuth tokens
type TokenPersister struct {
	db        *sql.DB
	encryptor *auth.EncryptionService
	logger    *logger.Logger
}

// NewTokenPersister creates a new token persister
func NewTokenPersister(db *sql.DB, encryptor *auth.EncryptionService, logger *logger.Logger) *TokenPersister {
	return &TokenPersister{
		db:        db,
		encryptor: encryptor,
		logger:    logger.WithContext("component", "token_persister"),
	}
}

// UpdateStravaTokens updates Strava OAuth tokens for a user
func (p *TokenPersister) UpdateStravaTokens(ctx context.Context, userID int, accessToken, refreshToken string, expiry time.Time) error {
	p.logger.Debug("Updating Strava tokens for user",
		"user_id", userID,
		"token_expiry", expiry,
		"has_access_token", accessToken != "",
		"has_refresh_token", refreshToken != "")

	// Encrypt tokens
	encryptedAccessToken, err := p.encryptor.Encrypt(accessToken)
	if err != nil {
		p.logger.Error("Failed to encrypt Strava access token",
			"error", err,
			"user_id", userID)
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encryptedRefreshToken, err := p.encryptor.Encrypt(refreshToken)
	if err != nil {
		p.logger.Error("Failed to encrypt Strava refresh token",
			"error", err,
			"user_id", userID)
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	// Update tokens in database
	query := `
		UPDATE users 
		SET strava_access_token = $1,
			strava_refresh_token = $2,
			strava_token_expiry = $3,
			updated_at = $4
		WHERE id = $5
	`

	result, err := p.db.ExecContext(ctx, query,
		encryptedAccessToken,
		encryptedRefreshToken,
		expiry,
		time.Now(),
		userID)

	if err != nil {
		p.logger.Error("Failed to update Strava tokens in database",
			"error", err,
			"user_id", userID)
		return fmt.Errorf("failed to update Strava tokens: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		p.logger.Warn("No user found to update Strava tokens",
			"user_id", userID)
		return fmt.Errorf("user %d not found", userID)
	}

	p.logger.Info("Successfully updated Strava tokens",
		"user_id", userID,
		"token_expiry", expiry,
		"token_valid_hours", time.Until(expiry).Hours())

	return nil
}

// UpdateGoogleTokens updates Google OAuth tokens for a user
func (p *TokenPersister) UpdateGoogleTokens(ctx context.Context, userID int, accessToken, refreshToken string, expiry time.Time) error {
	p.logger.Debug("Updating Google tokens for user",
		"user_id", userID,
		"token_expiry", expiry,
		"has_access_token", accessToken != "",
		"has_refresh_token", refreshToken != "")

	// Encrypt tokens
	encryptedAccessToken, err := p.encryptor.Encrypt(accessToken)
	if err != nil {
		p.logger.Error("Failed to encrypt Google access token",
			"error", err,
			"user_id", userID)
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encryptedRefreshToken, err := p.encryptor.Encrypt(refreshToken)
	if err != nil {
		p.logger.Error("Failed to encrypt Google refresh token",
			"error", err,
			"user_id", userID)
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	// Update tokens in database
	query := `
		UPDATE users 
		SET google_access_token = $1,
			google_refresh_token = $2,
			google_token_expiry = $3,
			updated_at = $4
		WHERE id = $5
	`

	result, err := p.db.ExecContext(ctx, query,
		encryptedAccessToken,
		encryptedRefreshToken,
		expiry,
		time.Now(),
		userID)

	if err != nil {
		p.logger.Error("Failed to update Google tokens in database",
			"error", err,
			"user_id", userID)
		return fmt.Errorf("failed to update Google tokens: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		p.logger.Warn("No user found to update Google tokens",
			"user_id", userID)
		return fmt.Errorf("user %d not found", userID)
	}

	p.logger.Info("Successfully updated Google tokens",
		"user_id", userID,
		"token_expiry", expiry,
		"token_valid_hours", time.Until(expiry).Hours())

	return nil
}
