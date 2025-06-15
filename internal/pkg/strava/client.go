package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// Activity represents a Strava activity with essential fields for automation processing
type Activity struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Type             string    `json:"type"`
	SportType        string    `json:"sport_type"`
	Distance         float64   `json:"distance"`         // meters
	MovingTime       int       `json:"moving_time"`      // seconds
	ElapsedTime      int       `json:"elapsed_time"`     // seconds
	TotalElevationGain float64 `json:"total_elevation_gain"` // meters
	StartDate        time.Time `json:"start_date"`
	StartDateLocal   time.Time `json:"start_date_local"`
	Timezone         string    `json:"timezone"`
	AverageSpeed     float64   `json:"average_speed"`    // meters per second
	MaxSpeed         float64   `json:"max_speed"`        // meters per second
	AverageHeartrate float64   `json:"average_heartrate"`
	MaxHeartrate     float64   `json:"max_heartrate"`
	Kudos            int       `json:"kudos_count"`
	Comments         int       `json:"comment_count"`
}

// Client provides Strava API access with automatic token lifecycle management
// This implements US023 requirements for managing Strava access tokens using refresh tokens
type Client struct {
	userID       int
	refreshToken string
	
	// In-memory token cache (per-job lifecycle)
	mu           sync.RWMutex
	accessToken  string
	tokenExpiry  time.Time
	
	// HTTP client for API requests
	httpClient *http.Client
	
	// OAuth configuration for token refresh
	oauthConfig *oauth2.Config
	
	// Logger for debugging external API interactions
	logger *logger.Logger
}

// NewClient creates a new Strava API client for a specific user
// The client is designed to be instantiated per-user for each processing job
func NewClient(userID int, refreshToken string, logger *logger.Logger) *Client {
	// Create OAuth2 config for token refresh operations
	oauthConfig := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.strava.com/oauth/authorize",
			TokenURL: "https://www.strava.com/oauth/token",
		},
		// Note: Client ID and Secret should be injected via config
		// For now, we'll set them when needed in token refresh
	}

	return &Client{
		userID:       userID,
		refreshToken: refreshToken,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		oauthConfig:  oauthConfig,
		logger:       logger.WithContext("component", "strava_client", "user_id", userID),
	}
}

// SetOAuthCredentials configures the OAuth client credentials for token refresh
// This should be called during client initialization with application credentials
func (c *Client) SetOAuthCredentials(clientID, clientSecret string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.oauthConfig.ClientID = clientID
	c.oauthConfig.ClientSecret = clientSecret
	
	c.logger.Debug("OAuth credentials configured for Strava client",
		"client_id", clientID,
		"has_client_secret", clientSecret != "")
}

// SetInitialTokens sets initial access token and expiry if available
// This allows the client to use existing valid tokens before falling back to refresh
func (c *Client) SetInitialTokens(accessToken string, expiry time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.accessToken = accessToken
	c.tokenExpiry = expiry
	
	c.logger.Debug("Initial access token set for Strava client",
		"has_access_token", accessToken != "",
		"token_expiry", expiry,
		"token_valid", time.Now().Before(expiry))
}

// ensureValidToken implements the "check-then-fetch" token management logic
// This method is called before every API request to guarantee a valid access token
func (c *Client) ensureValidToken(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.logger.Debug("Checking token validity before Strava API call",
		"has_access_token", c.accessToken != "",
		"token_expiry", c.tokenExpiry,
		"time_until_expiry_minutes", time.Until(c.tokenExpiry).Minutes())
	
	// Check if current access token is valid (with 5-minute buffer)
	if c.accessToken != "" && time.Now().Add(5*time.Minute).Before(c.tokenExpiry) {
		c.logger.Debug("Using existing valid access token for Strava API call",
			"token_expiry", c.tokenExpiry,
			"minutes_until_expiry", time.Until(c.tokenExpiry).Minutes())
		return nil
	}
	
	// Need to refresh token
	c.logger.Debug("Access token invalid or expired, refreshing via Strava OAuth",
		"current_token_expired", c.accessToken != "" && time.Now().After(c.tokenExpiry),
		"current_token_missing", c.accessToken == "",
		"refresh_token_available", c.refreshToken != "")
	
	if c.refreshToken == "" {
		c.logger.Error("No refresh token available for Strava token refresh",
			"user_id", c.userID)
		return ErrReauthRequired
	}
	
	// Call Strava OAuth token endpoint to refresh access token
	startTime := time.Now()
	c.logger.Debug("Making token refresh request to Strava OAuth endpoint",
		"endpoint", c.oauthConfig.Endpoint.TokenURL,
		"user_id", c.userID)
	
	token := &oauth2.Token{
		RefreshToken: c.refreshToken,
	}
	
	tokenSource := c.oauthConfig.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	
	requestDuration := time.Since(startTime)
	
	if err != nil {
		c.logger.Error("Failed to refresh Strava access token via OAuth endpoint",
			"error", err,
			"user_id", c.userID,
			"request_duration_ms", requestDuration.Milliseconds(),
			"endpoint", c.oauthConfig.Endpoint.TokenURL)
		
		// Check if this is an invalid grant error (requires re-authorization)
		if IsReauthRequired(err) {
			c.logger.Warn("Strava refresh token is invalid, user re-authorization required",
				"user_id", c.userID,
				"error", err)
			return &AuthError{
				Type:    "REAUTH_REQUIRED",
				Message: "Strava refresh token is invalid, user must re-authorize",
				Cause:   err,
			}
		}
		
		return &NetworkError{
			Operation: "token_refresh",
			Message:   "Failed to refresh Strava access token",
			Cause:     err,
		}
	}
	
	// Update cached token
	c.accessToken = newToken.AccessToken
	c.tokenExpiry = newToken.Expiry
	
	c.logger.Info("Successfully refreshed Strava access token",
		"user_id", c.userID,
		"new_token_expiry", newToken.Expiry,
		"token_valid_hours", time.Until(newToken.Expiry).Hours(),
		"refresh_duration_ms", requestDuration.Milliseconds())
	
	return nil
}

// makeAPIRequest performs an authenticated HTTP request to the Strava API
// This method includes comprehensive logging for debugging external API interactions
func (c *Client) makeAPIRequest(ctx context.Context, method, endpoint string, result interface{}) error {
	// Ensure we have a valid access token
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}
	
	// Build full URL
	url := "https://www.strava.com/api/v3" + endpoint
	
	startTime := time.Now()
	c.logger.Debug("Making Strava API request",
		"method", method,
		"endpoint", endpoint,
		"full_url", url,
		"user_id", c.userID)
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		c.logger.Error("Failed to create Strava API request",
			"error", err,
			"method", method,
			"endpoint", endpoint,
			"user_id", c.userID)
		return &NetworkError{
			Operation: "request_creation",
			Message:   "Failed to create HTTP request",
			Cause:     err,
		}
	}
	
	// Add authorization header
	c.mu.RLock()
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	c.mu.RUnlock()
	
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Academy-Sync-Automation/1.0")
	
	// Execute request
	c.logger.Debug("Executing HTTP request to Strava API",
		"method", method,
		"url", url,
		"headers", map[string]string{
			"Accept":     req.Header.Get("Accept"),
			"User-Agent": req.Header.Get("User-Agent"),
		})
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		requestDuration := time.Since(startTime)
		c.logger.Error("Strava API request failed with network error",
			"error", err,
			"method", method,
			"endpoint", endpoint,
			"user_id", c.userID,
			"request_duration_ms", requestDuration.Milliseconds())
		return &NetworkError{
			Operation: "api_request",
			Message:   "Network error during API request",
			Cause:     err,
		}
	}
	defer resp.Body.Close()
	
	requestDuration := time.Since(startTime)
	
	c.logger.Debug("Received response from Strava API",
		"method", method,
		"endpoint", endpoint,
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"content_length", resp.ContentLength,
		"content_type", resp.Header.Get("Content-Type"),
		"user_id", c.userID,
		"request_duration_ms", requestDuration.Milliseconds())
	
	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to read error details from response body
		var errorDetails map[string]interface{}
		if decodeErr := json.NewDecoder(resp.Body).Decode(&errorDetails); decodeErr == nil {
			c.logger.Error("Strava API returned error response with details",
				"status_code", resp.StatusCode,
				"status", resp.Status,
				"error_details", errorDetails,
				"method", method,
				"endpoint", endpoint,
				"user_id", c.userID,
				"request_duration_ms", requestDuration.Milliseconds())
		} else {
			c.logger.Error("Strava API returned error response",
				"status_code", resp.StatusCode,
				"status", resp.Status,
				"method", method,
				"endpoint", endpoint,
				"user_id", c.userID,
				"request_duration_ms", requestDuration.Milliseconds(),
				"decode_error", decodeErr)
		}
		
		// Handle specific error codes
		switch resp.StatusCode {
		case 401:
			c.logger.Warn("Strava API returned 401 Unauthorized, access token may be invalid",
				"user_id", c.userID,
				"endpoint", endpoint)
			return &AuthError{
				Type:    "ACCESS_DENIED",
				Message: "Strava API access denied, token may be invalid",
			}
		case 403:
			return &AuthError{
				Type:    "FORBIDDEN",
				Message: "Strava API access forbidden, insufficient permissions",
			}
		case 429:
			return &APIError{
				StatusCode: resp.StatusCode,
				Type:       "RATE_LIMITED",
				Message:    "Strava API rate limit exceeded",
			}
		default:
			return &APIError{
				StatusCode: resp.StatusCode,
				Type:       "HTTP_ERROR",
				Message:    fmt.Sprintf("Strava API error: %s", resp.Status),
			}
		}
	}
	
	// Decode successful response
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		c.logger.Error("Failed to decode Strava API response",
			"error", err,
			"method", method,
			"endpoint", endpoint,
			"status_code", resp.StatusCode,
			"content_type", resp.Header.Get("Content-Type"),
			"user_id", c.userID,
			"request_duration_ms", requestDuration.Milliseconds())
		return &APIError{
			StatusCode: resp.StatusCode,
			Type:       "DECODE_ERROR",
			Message:    "Failed to decode API response",
			Cause:      err,
		}
	}
	
	c.logger.Info("Successfully completed Strava API request",
		"method", method,
		"endpoint", endpoint,
		"status_code", resp.StatusCode,
		"user_id", c.userID,
		"request_duration_ms", requestDuration.Milliseconds())
	
	return nil
}

// GetActivities retrieves activities from Strava after a specified time
// This implements the core functionality needed for automation processing
func (c *Client) GetActivities(ctx context.Context, after time.Time) ([]Activity, error) {
	c.logger.Debug("Retrieving activities from Strava",
		"user_id", c.userID,
		"after", after.Format(time.RFC3339),
		"days_back", time.Since(after).Hours()/24)
	
	// Build query parameters
	afterUnix := after.Unix()
	endpoint := fmt.Sprintf("/athlete/activities?after=%d&per_page=100", afterUnix)
	
	var activities []Activity
	if err := c.makeAPIRequest(ctx, "GET", endpoint, &activities); err != nil {
		c.logger.Error("Failed to retrieve activities from Strava",
			"error", err,
			"user_id", c.userID,
			"after", after.Format(time.RFC3339))
		return nil, err
	}
	
	c.logger.Info("Successfully retrieved activities from Strava",
		"user_id", c.userID,
		"activity_count", len(activities),
		"after", after.Format(time.RFC3339),
		"first_activity", func() string {
			if len(activities) > 0 {
				return activities[0].StartDate.Format(time.RFC3339)
			}
			return "none"
		}())
	
	return activities, nil
}

// GetActivity retrieves a specific activity by ID from Strava
func (c *Client) GetActivity(ctx context.Context, activityID int64) (*Activity, error) {
	c.logger.Debug("Retrieving specific activity from Strava",
		"user_id", c.userID,
		"activity_id", activityID)
	
	endpoint := fmt.Sprintf("/activities/%d", activityID)
	
	var activity Activity
	if err := c.makeAPIRequest(ctx, "GET", endpoint, &activity); err != nil {
		c.logger.Error("Failed to retrieve activity from Strava",
			"error", err,
			"user_id", c.userID,
			"activity_id", activityID)
		return nil, err
	}
	
	c.logger.Info("Successfully retrieved activity from Strava",
		"user_id", c.userID,
		"activity_id", activityID,
		"activity_name", activity.Name,
		"activity_type", activity.Type,
		"activity_date", activity.StartDate.Format(time.RFC3339))
	
	return &activity, nil
}

// GetAthleteProfile retrieves the authenticated athlete's profile information
func (c *Client) GetAthleteProfile(ctx context.Context) (map[string]interface{}, error) {
	c.logger.Debug("Retrieving athlete profile from Strava",
		"user_id", c.userID)
	
	var profile map[string]interface{}
	if err := c.makeAPIRequest(ctx, "GET", "/athlete", &profile); err != nil {
		c.logger.Error("Failed to retrieve athlete profile from Strava",
			"error", err,
			"user_id", c.userID)
		return nil, err
	}
	
	// Extract athlete ID for logging (if available)
	athleteID := "unknown"
	if id, ok := profile["id"]; ok {
		athleteID = fmt.Sprintf("%v", id)
	}
	
	c.logger.Info("Successfully retrieved athlete profile from Strava",
		"user_id", c.userID,
		"athlete_id", athleteID,
		"profile_fields", len(profile))
	
	return profile, nil
}