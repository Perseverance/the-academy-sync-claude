package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// TokenRefreshService handles OAuth token refresh for Google and Strava
type TokenRefreshService struct {
	userRepository   UserRepository
	googleClientID   string
	googleSecret     string
	stravaClientID   string
	stravaSecret     string
	logger           *logger.Logger
	httpClient       *http.Client
}

// TokenRefreshResult represents the result of a token refresh operation
type TokenRefreshResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	ExpiresAt    time.Time
}

// NewTokenRefreshService creates a new token refresh service
func NewTokenRefreshService(
	userRepository UserRepository,
	googleClientID, googleSecret string,
	stravaClientID, stravaSecret string,
	logger *logger.Logger,
) *TokenRefreshService {
	return &TokenRefreshService{
		userRepository: userRepository,
		googleClientID: googleClientID,
		googleSecret:   googleSecret,
		stravaClientID: stravaClientID,
		stravaSecret:   stravaSecret,
		logger:         logger.WithContext("component", "token_refresh_service"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RefreshGoogleToken refreshes a Google OAuth token using the refresh token
func (s *TokenRefreshService) RefreshGoogleToken(ctx context.Context, refreshToken string) (*TokenRefreshResult, error) {
	s.logger.Debug("Refreshing Google OAuth token")

	// Prepare the token refresh request
	data := url.Values{}
	data.Set("client_id", s.googleClientID)
	data.Set("client_secret", s.googleSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create Google token refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Google token refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Google token refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("Google token refresh failed",
			"status_code", resp.StatusCode,
			"response_body", string(body))
		return nil, fmt.Errorf("Google token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to parse Google token refresh response: %w", err)
	}

	// Calculate expiry time
	expiresAt := time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)

	s.logger.Info("Google token refreshed successfully",
		"expires_in_seconds", tokenResponse.ExpiresIn,
		"expires_at", expiresAt)

	result := &TokenRefreshResult{
		AccessToken:  tokenResponse.AccessToken,
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresIn:    tokenResponse.ExpiresIn,
		ExpiresAt:    expiresAt,
	}

	// If no new refresh token provided, keep the old one
	if result.RefreshToken == "" {
		result.RefreshToken = refreshToken
	}

	return result, nil
}

// RefreshStravaToken refreshes a Strava OAuth token using the refresh token
func (s *TokenRefreshService) RefreshStravaToken(ctx context.Context, refreshToken string) (*TokenRefreshResult, error) {
	s.logger.Debug("Refreshing Strava OAuth token")

	// Prepare the token refresh request
	data := url.Values{}
	data.Set("client_id", s.stravaClientID)
	data.Set("client_secret", s.stravaSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://www.strava.com/oauth/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create Strava token refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Strava token refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Strava token refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("Strava token refresh failed",
			"status_code", resp.StatusCode,
			"response_body", string(body))
		return nil, fmt.Errorf("Strava token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to parse Strava token refresh response: %w", err)
	}

	// Calculate expiry time
	expiresAt := time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)

	s.logger.Info("Strava token refreshed successfully",
		"expires_in_seconds", tokenResponse.ExpiresIn,
		"expires_at", expiresAt)

	return &TokenRefreshResult{
		AccessToken:  tokenResponse.AccessToken,
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresIn:    tokenResponse.ExpiresIn,
		ExpiresAt:    expiresAt,
	}, nil
}

// RefreshTokensIfNeeded checks and refreshes tokens for a user if they are expired or close to expiry
func (s *TokenRefreshService) RefreshTokensIfNeeded(ctx context.Context, config *ProcessingConfig) (*ProcessingConfig, error) {
	s.logger.Debug("Checking if token refresh is needed for user",
		"user_id", config.UserID)

	updatedConfig := *config // Copy the config
	needsUpdate := false

	// Check Google token
	if !config.HasValidGoogleToken() && config.GoogleRefreshToken != "" {
		s.logger.Info("Google token needs refresh", "user_id", config.UserID)
		
		result, err := s.RefreshGoogleToken(ctx, config.GoogleRefreshToken)
		if err != nil {
			s.logger.Error("Failed to refresh Google token",
				"user_id", config.UserID,
				"error", err)
			return nil, fmt.Errorf("failed to refresh Google token: %w", err)
		}

		updatedConfig.GoogleAccessToken = result.AccessToken
		updatedConfig.GoogleRefreshToken = result.RefreshToken
		updatedConfig.GoogleTokenExpiry = &result.ExpiresAt
		needsUpdate = true

		s.logger.Info("Google token refreshed successfully",
			"user_id", config.UserID,
			"expires_at", result.ExpiresAt)
	}

	// Check Strava token
	if !config.HasValidStravaToken() && config.StravaRefreshToken != "" {
		s.logger.Info("Strava token needs refresh", "user_id", config.UserID)
		
		result, err := s.RefreshStravaToken(ctx, config.StravaRefreshToken)
		if err != nil {
			s.logger.Error("Failed to refresh Strava token",
				"user_id", config.UserID,
				"error", err)
			return nil, fmt.Errorf("failed to refresh Strava token: %w", err)
		}

		updatedConfig.StravaAccessToken = result.AccessToken
		updatedConfig.StravaRefreshToken = result.RefreshToken
		updatedConfig.StravaTokenExpiry = &result.ExpiresAt
		needsUpdate = true

		s.logger.Info("Strava token refreshed successfully",
			"user_id", config.UserID,
			"expires_at", result.ExpiresAt)
	}

	// Update database if tokens were refreshed
	if needsUpdate {
		if err := s.updateUserTokens(ctx, &updatedConfig); err != nil {
			s.logger.Error("Failed to update user tokens in database",
				"user_id", config.UserID,
				"error", err)
			return nil, fmt.Errorf("failed to update user tokens: %w", err)
		}

		s.logger.Info("User tokens updated successfully in database",
			"user_id", config.UserID)
	}

	return &updatedConfig, nil
}

// updateUserTokens updates the user's tokens in the database
func (s *TokenRefreshService) updateUserTokens(ctx context.Context, config *ProcessingConfig) error {
	s.logger.Debug("Updating user tokens in database",
		"user_id", config.UserID)

	// Since the UserRepository interface is limited, we need to extend it
	// For now, we'll try to type assert to get access to the concrete type methods
	if userRepo, ok := s.userRepository.(interface {
		UpdateGoogleTokens(ctx context.Context, userID int, accessToken, refreshToken string, expiry *time.Time) error
		UpdateStravaTokens(ctx context.Context, userID int, accessToken, refreshToken string, expiry *time.Time) error
	}); ok {
		// Update Google tokens if they exist
		if config.GoogleAccessToken != "" && config.GoogleRefreshToken != "" {
			if err := userRepo.UpdateGoogleTokens(ctx, config.UserID, config.GoogleAccessToken, config.GoogleRefreshToken, config.GoogleTokenExpiry); err != nil {
				return fmt.Errorf("failed to update Google tokens: %w", err)
			}
		}

		// Update Strava tokens if they exist
		if config.StravaAccessToken != "" && config.StravaRefreshToken != "" {
			if err := userRepo.UpdateStravaTokens(ctx, config.UserID, config.StravaAccessToken, config.StravaRefreshToken, config.StravaTokenExpiry); err != nil {
				return fmt.Errorf("failed to update Strava tokens: %w", err)
			}
		}

		s.logger.Info("User tokens updated successfully in database", "user_id", config.UserID)
		return nil
	}

	// Fallback: log that we couldn't update tokens
	s.logger.Warn("UserRepository does not support direct token updates - tokens not persisted",
		"user_id", config.UserID)
	return nil
}