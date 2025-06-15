package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleUserInfo represents the user information returned by Google's userinfo API
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// StravaUserInfo represents the athlete information returned by Strava's API
type StravaUserInfo struct {
	ID        int64  `json:"id"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Profile   string `json:"profile"`
	City      string `json:"city"`
	State     string `json:"state"`
	Country   string `json:"country"`
}

// OAuthService handles OAuth 2.0 authentication for Google and Strava
type OAuthService struct {
	googleConfig *oauth2.Config
	stravaConfig *oauth2.Config
}

// NewOAuthService creates a new OAuth service with Google and Strava configurations
func NewOAuthService(googleClientID, googleClientSecret, googleRedirectURL, stravaClientID, stravaClientSecret, stravaRedirectURL string) *OAuthService {
	googleConfig := &oauth2.Config{
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		RedirectURL:  googleRedirectURL,
		Scopes: []string{
			"openid",
			"profile",
			"email",
			"https://www.googleapis.com/auth/spreadsheets", // Google Sheets access for automation
		},
		Endpoint: google.Endpoint,
	}

	stravaConfig := &oauth2.Config{
		ClientID:     stravaClientID,
		ClientSecret: stravaClientSecret,
		RedirectURL:  stravaRedirectURL,
		Scopes: []string{
			"activity:read_all", // Read all activities from Strava
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.strava.com/oauth/authorize",
			TokenURL: "https://www.strava.com/api/v3/oauth/token",
		},
	}

	return &OAuthService{
		googleConfig: googleConfig,
		stravaConfig: stravaConfig,
	}
}

// GetAuthURL generates the Google OAuth authorization URL
func (o *OAuthService) GetAuthURL(state string) string {
	return o.googleConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
}

// GetStravaAuthURL generates the Strava OAuth authorization URL
func (o *OAuthService) GetStravaAuthURL(state string) string {
	return o.stravaConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("approval_prompt", "force"))
}

// ExchangeCodeForToken exchanges an authorization code for Google OAuth tokens
func (o *OAuthService) ExchangeCodeForToken(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := o.googleConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	return token, nil
}

// ExchangeStravaCodeForToken exchanges an authorization code for Strava OAuth tokens
func (o *OAuthService) ExchangeStravaCodeForToken(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := o.stravaConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange Strava code for token: %w", err)
	}

	return token, nil
}

// GetUserInfo retrieves user information from Google using the access token
func (o *OAuthService) GetUserInfo(ctx context.Context, token *oauth2.Token) (*GoogleUserInfo, error) {
	client := o.googleConfig.Client(ctx, token)

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user info: status %d", resp.StatusCode)
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	// Validate that email is verified
	if !userInfo.VerifiedEmail {
		return nil, fmt.Errorf("user email is not verified")
	}

	return &userInfo, nil
}

// GetStravaUserInfo retrieves athlete information from Strava using the access token
func (o *OAuthService) GetStravaUserInfo(ctx context.Context, token *oauth2.Token) (*StravaUserInfo, error) {
	client := o.stravaConfig.Client(ctx, token)

	resp, err := client.Get("https://www.strava.com/api/v3/athlete")
	if err != nil {
		return nil, fmt.Errorf("failed to get Strava athlete info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get Strava athlete info: status %d", resp.StatusCode)
	}

	var athleteInfo StravaUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&athleteInfo); err != nil {
		return nil, fmt.Errorf("failed to decode Strava athlete info: %w", err)
	}

	// Validate that we got essential athlete information
	if athleteInfo.ID == 0 {
		return nil, fmt.Errorf("invalid athlete info: missing athlete ID")
	}

	return &athleteInfo, nil
}

// RefreshToken refreshes a Google OAuth token using the refresh token
func (o *OAuthService) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	tokenSource := o.googleConfig.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh Google token: %w", err)
	}

	return newToken, nil
}

// RefreshStravaToken refreshes a Strava OAuth token using the refresh token
func (o *OAuthService) RefreshStravaToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	tokenSource := o.stravaConfig.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh Strava token: %w", err)
	}

	return newToken, nil
}