package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// TokenRefreshService handles automatic token refresh for Google and Strava OAuth tokens
type TokenRefreshService struct {
	userRepository   *database.UserRepository
	googleClientID   string
	googleClientSecret string
	stravaClientID   string
	stravaClientSecret string
	logger           *logger.Logger
	httpClient       *http.Client
}

// NewTokenRefreshService creates a new token refresh service
func NewTokenRefreshService(
	userRepository *database.UserRepository,
	googleClientID, googleClientSecret string,
	stravaClientID, stravaClientSecret string,
	logger *logger.Logger,
) *TokenRefreshService {
	return &TokenRefreshService{
		userRepository:     userRepository,
		googleClientID:     googleClientID,
		googleClientSecret: googleClientSecret,
		stravaClientID:     stravaClientID,
		stravaClientSecret: stravaClientSecret,
		logger:             logger.WithContext("component", "token_refresh_service"),
		httpClient:         &http.Client{Timeout: 30 * time.Second},
	}
}

// GoogleTokenResponse represents the response from Google's token refresh endpoint
type GoogleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"` // Only provided if refresh token rotation is enabled
	TokenType    string `json:"token_type"`
}

// StravaTokenResponse represents the response from Strava's token refresh endpoint
type StravaTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	ExpiresIn    int    `json:"expires_in"`
}

// RefreshGoogleToken refreshes an expired Google OAuth token using the refresh token
func (s *TokenRefreshService) RefreshGoogleToken(ctx context.Context, userID int, refreshToken string) (*GoogleTokenResponse, error) {
	s.logger.Info("Refreshing Google OAuth token",
		"user_id", userID)

	// Prepare the refresh request
	data := url.Values{}
	data.Set("client_id", s.googleClientID)
	data.Set("client_secret", s.googleClientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Error("Failed to make Google token refresh request",
			"error", err,
			"user_id", userID)
		return nil, fmt.Errorf("failed to make refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("Google token refresh failed",
			"status_code", resp.StatusCode,
			"user_id", userID)
		return nil, fmt.Errorf("token refresh failed with status %d", resp.StatusCode)
	}

	// Parse the response
	var tokenResponse GoogleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		s.logger.Error("Failed to parse Google token refresh response",
			"error", err,
			"user_id", userID)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	s.logger.Info("Successfully refreshed Google OAuth token",
		"user_id", userID,
		"new_expiry_seconds", tokenResponse.ExpiresIn)

	return &tokenResponse, nil
}

// RefreshStravaToken refreshes an expired Strava OAuth token using the refresh token
func (s *TokenRefreshService) RefreshStravaToken(ctx context.Context, userID int, refreshToken string) (*StravaTokenResponse, error) {
	s.logger.Info("Refreshing Strava OAuth token",
		"user_id", userID)

	// Prepare the refresh request
	data := url.Values{}
	data.Set("client_id", s.stravaClientID)
	data.Set("client_secret", s.stravaClientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://www.strava.com/oauth/token", bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Error("Failed to make Strava token refresh request",
			"error", err,
			"user_id", userID)
		return nil, fmt.Errorf("failed to make refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("Strava token refresh failed",
			"status_code", resp.StatusCode,
			"user_id", userID)
		return nil, fmt.Errorf("token refresh failed with status %d", resp.StatusCode)
	}

	// Parse the response
	var tokenResponse StravaTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		s.logger.Error("Failed to parse Strava token refresh response",
			"error", err,
			"user_id", userID)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	s.logger.Info("Successfully refreshed Strava OAuth token",
		"user_id", userID,
		"new_expiry_timestamp", tokenResponse.ExpiresAt)

	return &tokenResponse, nil
}

// RefreshUserTokensIfNeeded checks if tokens are expired and refreshes them if necessary
func (s *TokenRefreshService) RefreshUserTokensIfNeeded(ctx context.Context, config *ProcessingConfig) (*ProcessingConfig, error) {
	s.logger.Debug("Checking if token refresh is needed",
		"user_id", config.UserID,
		"google_token_valid", config.HasValidGoogleToken(),
		"strava_token_valid", config.HasValidStravaToken())

	refreshedConfig := *config // Create a copy
	tokensRefreshed := false

	// Check and refresh Google token if needed
	if !config.HasValidGoogleToken() && config.GoogleRefreshToken != "" {
		s.logger.Info("Google token expired, attempting refresh",
			"user_id", config.UserID)

		googleToken, err := s.RefreshGoogleToken(ctx, config.UserID, config.GoogleRefreshToken)
		if err != nil {
			s.logger.Error("Failed to refresh Google token",
				"error", err,
				"user_id", config.UserID)
			return nil, fmt.Errorf("failed to refresh Google token: %w", err)
		}

		// Update the config with new tokens
		refreshedConfig.GoogleAccessToken = googleToken.AccessToken
		newExpiry := time.Now().Add(time.Duration(googleToken.ExpiresIn) * time.Second)
		refreshedConfig.GoogleTokenExpiry = &newExpiry

		// Update refresh token if provided (Google may rotate refresh tokens)
		if googleToken.RefreshToken != "" {
			refreshedConfig.GoogleRefreshToken = googleToken.RefreshToken
		}

		// Save to database
		updateReq := &database.UpdateUserTokensRequest{
			UserID:             config.UserID,
			GoogleAccessToken:  googleToken.AccessToken,
			GoogleRefreshToken: refreshedConfig.GoogleRefreshToken,
			GoogleTokenExpiry:  &newExpiry,
		}

		if err := s.userRepository.UpdateUserTokens(ctx, updateReq); err != nil {
			s.logger.Error("Failed to save refreshed Google token to database",
				"error", err,
				"user_id", config.UserID)
			return nil, fmt.Errorf("failed to save refreshed Google token: %w", err)
		}

		tokensRefreshed = true
		s.logger.Info("Successfully refreshed and saved Google token",
			"user_id", config.UserID)
	}

	// Check and refresh Strava token if needed
	if !config.HasValidStravaToken() && config.StravaRefreshToken != "" {
		s.logger.Info("Strava token expired, attempting refresh",
			"user_id", config.UserID)

		stravaToken, err := s.RefreshStravaToken(ctx, config.UserID, config.StravaRefreshToken)
		if err != nil {
			s.logger.Error("Failed to refresh Strava token",
				"error", err,
				"user_id", config.UserID)
			return nil, fmt.Errorf("failed to refresh Strava token: %w", err)
		}

		// Update the config with new tokens
		refreshedConfig.StravaAccessToken = stravaToken.AccessToken
		refreshedConfig.StravaRefreshToken = stravaToken.RefreshToken
		newExpiry := time.Unix(stravaToken.ExpiresAt, 0)
		refreshedConfig.StravaTokenExpiry = &newExpiry

		// Save to database
		if err := s.userRepository.UpdateStravaTokensOnly(ctx, config.UserID, stravaToken.AccessToken, stravaToken.RefreshToken, &newExpiry); err != nil {
			s.logger.Error("Failed to save refreshed Strava token to database",
				"error", err,
				"user_id", config.UserID)
			return nil, fmt.Errorf("failed to save refreshed Strava token: %w", err)
		}

		tokensRefreshed = true
		s.logger.Info("Successfully refreshed Strava token",
			"user_id", config.UserID)
	}

	if tokensRefreshed {
		s.logger.Info("Token refresh completed",
			"user_id", config.UserID,
			"google_refreshed", !config.HasValidGoogleToken(),
			"strava_refreshed", !config.HasValidStravaToken())
	}

	return &refreshedConfig, nil
}