package database

import (
	"fmt"
	"time"
)

// User represents a user in the system
type User struct {
	ID                        int        `json:"id" db:"id"`
	GoogleID                  string     `json:"google_id" db:"google_id"`
	Email                     string     `json:"email" db:"email"`
	Name                      string     `json:"name" db:"name"`
	ProfilePictureURL         *string    `json:"profile_picture_url" db:"profile_picture_url"`
	GoogleAccessToken         []byte     `json:"-" db:"google_access_token"`  // Encrypted, never sent to client
	GoogleRefreshToken        []byte     `json:"-" db:"google_refresh_token"` // Encrypted, never sent to client
	GoogleTokenExpiry         *time.Time `json:"-" db:"google_token_expiry"`  // Never sent to client
	StravaAccessToken         []byte     `json:"-" db:"strava_access_token"`  // Encrypted, never sent to client
	StravaRefreshToken        []byte     `json:"-" db:"strava_refresh_token"` // Encrypted, never sent to client
	StravaTokenExpiry         *time.Time `json:"-" db:"strava_token_expiry"`  // Never sent to client
	StravaAthleteID           *int64     `json:"strava_athlete_id" db:"strava_athlete_id"`
	StravaAthleteName         *string    `json:"strava_athlete_name" db:"strava_athlete_name"`
	StravaProfilePictureURL   *string    `json:"strava_profile_picture_url" db:"strava_profile_picture_url"`
	SpreadsheetID             *string    `json:"spreadsheet_id" db:"spreadsheet_id"`
	Timezone                  string     `json:"timezone" db:"timezone"`
	EmailNotificationsEnabled bool       `json:"email_notifications_enabled" db:"email_notifications_enabled"`
	AutomationEnabled         bool       `json:"automation_enabled" db:"automation_enabled"`
	CreatedAt                 time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at" db:"updated_at"`
	LastLoginAt               *time.Time `json:"last_login_at" db:"last_login_at"`
}

// UserSession represents a user session in the system
type UserSession struct {
	ID           int       `json:"id" db:"id"`
	UserID       int       `json:"user_id" db:"user_id"`
	SessionToken string    `json:"-" db:"session_token"` // Never sent to client
	UserAgent    *string   `json:"user_agent" db:"user_agent"`
	IPAddress    *string   `json:"ip_address" db:"ip_address"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	LastUsedAt   time.Time `json:"last_used_at" db:"last_used_at"`
	IsActive     bool      `json:"is_active" db:"is_active"`
}

// CreateUserRequest represents the data needed to create a new user
type CreateUserRequest struct {
	GoogleID           string
	Email              string
	Name               string
	ProfilePictureURL  *string
	GoogleAccessToken  string
	GoogleRefreshToken string
	GoogleTokenExpiry  *time.Time
}

// UpdateUserTokensRequest represents the data needed to update user's OAuth tokens
type UpdateUserTokensRequest struct {
	UserID             int
	GoogleAccessToken  string
	GoogleRefreshToken string
	GoogleTokenExpiry  *time.Time
	UpdateLastLogin    bool // If true, also updates last_login_at
}

// CreateSessionRequest represents the data needed to create a new session
type CreateSessionRequest struct {
	UserID       int
	SessionToken string
	UserAgent    *string
	IPAddress    *string
	ExpiresAt    time.Time
}

// PublicUser represents user data safe to send to the client
type PublicUser struct {
	ID                        int     `json:"id"`
	Email                     string  `json:"email"`
	Name                      string  `json:"name"`
	ProfilePictureURL         *string `json:"profile_picture_url"`
	StravaAthleteID           *int64  `json:"strava_athlete_id"`
	StravaAthleteName         *string `json:"strava_athlete_name"`
	StravaProfilePictureURL   *string `json:"strava_profile_picture_url"`
	SpreadsheetID             *string `json:"spreadsheet_id"`
	Timezone                  string  `json:"timezone"`
	EmailNotificationsEnabled bool    `json:"email_notifications_enabled"`
	AutomationEnabled         bool    `json:"automation_enabled"`
	HasStravaConnection       bool    `json:"has_strava_connection"`
	HasSheetsConnection       bool    `json:"has_sheets_connection"`
}

// ToPublicUser converts a User to PublicUser, removing sensitive information
func (u *User) ToPublicUser() *PublicUser {
	return &PublicUser{
		ID:                        u.ID,
		Email:                     u.Email,
		Name:                      u.Name,
		ProfilePictureURL:         u.ProfilePictureURL,
		StravaAthleteID:           u.StravaAthleteID,
		StravaAthleteName:         u.StravaAthleteName,
		StravaProfilePictureURL:   u.StravaProfilePictureURL,
		SpreadsheetID:             u.SpreadsheetID,
		Timezone:                  u.Timezone,
		EmailNotificationsEnabled: u.EmailNotificationsEnabled,
		AutomationEnabled:         u.AutomationEnabled,
		HasStravaConnection:       len(u.StravaAccessToken) > 0,
		HasSheetsConnection:       u.SpreadsheetID != nil && *u.SpreadsheetID != "",
	}
}

// ActivityLog represents an activity log entry in the database
type ActivityLog struct {
	ID                      int                    `json:"id" db:"id"`
	UserID                  int                    `json:"user_id" db:"user_id"`
	ProcessingDate          time.Time              `json:"processing_date" db:"processing_date"`
	ProcessingType          string                 `json:"processing_type" db:"processing_type"`
	ProcessingScope         string                 `json:"processing_scope" db:"processing_scope"`
	Status                  string                 `json:"status" db:"status"`
	ActivitiesFound         int                    `json:"activities_found" db:"activities_found"`
	ActivitiesProcessed     int                    `json:"activities_processed" db:"activities_processed"`
	TotalDistanceMeters     *float64               `json:"total_distance_meters" db:"total_distance_meters"`
	TotalDurationSeconds    *int                   `json:"total_duration_seconds" db:"total_duration_seconds"`
	SpreadsheetRow          *int                   `json:"spreadsheet_row" db:"spreadsheet_row"`
	SpreadsheetUpdated      bool                   `json:"spreadsheet_updated" db:"spreadsheet_updated"`
	DescriptionGenerated    *string                `json:"description_generated" db:"description_generated"`
	ErrorMessage            *string                `json:"error_message" db:"error_message"`
	WarningMessages         []string               `json:"warning_messages" db:"warning_messages"`
	ProcessingStartedAt     time.Time              `json:"processing_started_at" db:"processing_started_at"`
	ProcessingCompletedAt   *time.Time             `json:"processing_completed_at" db:"processing_completed_at"`
	ProcessingDurationMs    *int                   `json:"processing_duration_ms" db:"processing_duration_ms"`
	Metadata                map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt               time.Time              `json:"created_at" db:"created_at"`
}

// ActivityLogSummary represents a simplified activity log for the dashboard UI
type ActivityLogSummary struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Status  string `json:"status"` // "Success", "Failure", "SuccessWithWarning"
	Summary string `json:"summary"`
}

// ToSummary converts an ActivityLog to ActivityLogSummary for UI display
func (a *ActivityLog) ToSummary() ActivityLogSummary {
	// Determine UI status based on database status and warnings
	uiStatus := "Success"
	if a.Status == "failed" {
		uiStatus = "Failure"
	} else if a.Status == "success" && len(a.WarningMessages) > 0 {
		uiStatus = "SuccessWithWarning"
	}

	// Generate summary text
	summary := a.generateSummary()

	return ActivityLogSummary{
		ID:      fmt.Sprintf("log-%d", a.ID),
		Date:    a.CreatedAt.Format(time.RFC3339),
		Status:  uiStatus,
		Summary: summary,
	}
}

// generateSummary creates a human-readable summary of the processing result
func (a *ActivityLog) generateSummary() string {
	if a.Status == "failed" && a.ErrorMessage != nil {
		return *a.ErrorMessage
	}

	if a.Status == "skipped" {
		if len(a.WarningMessages) > 0 {
			return a.WarningMessages[0]
		}
		return "Processing skipped"
	}

	// Success case
	summary := fmt.Sprintf("Processed %s: ", a.ProcessingDate.Format("2006-01-02"))
	
	if a.ActivitiesFound == 0 {
		summary += "No activities found"
	} else if a.ActivitiesFound == 1 {
		summary += "1 activity found"
	} else {
		summary += fmt.Sprintf("%d activities found", a.ActivitiesFound)
	}

	if a.ActivitiesProcessed > 0 {
		summary += fmt.Sprintf(", %d processed", a.ActivitiesProcessed)
	}

	if a.SpreadsheetUpdated {
		summary += ", spreadsheet updated"
	}

	return summary
}

// DashboardUserResponse represents user data with dashboard-specific additions
type DashboardUserResponse struct {
	*PublicUser
	RecentActivityLogs []ActivityLogSummary `json:"recent_activity_logs"`
}
