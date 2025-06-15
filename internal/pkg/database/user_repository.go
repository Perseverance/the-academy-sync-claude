package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
)

// UserRepository handles database operations for users
type UserRepository struct {
	db        *sql.DB
	encryptor *auth.EncryptionService
}

// ProcessingTokens contains all decrypted tokens and configuration needed for automation processing
type ProcessingTokens struct {
	GoogleAccessToken  string
	GoogleRefreshToken string
	GoogleTokenExpiry  *time.Time
	StravaAccessToken  string
	StravaRefreshToken string
	StravaTokenExpiry  *time.Time
	StravaAthleteID    *int64
	SpreadsheetID      *string
	Timezone           string
	Email              string
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sql.DB, encryptor *auth.EncryptionService) *UserRepository {
	return &UserRepository{
		db:        db,
		encryptor: encryptor,
	}
}

// CreateUser creates a new user in the database
func (r *UserRepository) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	// Encrypt OAuth tokens
	encryptedAccessToken, err := r.encryptor.Encrypt(req.GoogleAccessToken)
	if err != nil {
		return nil, err
	}

	encryptedRefreshToken, err := r.encryptor.Encrypt(req.GoogleRefreshToken)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO users (
			google_id, email, name, profile_picture_url,
			google_access_token, google_refresh_token, google_token_expiry,
			created_at, updated_at, last_login_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	now := time.Now()
	var user User
	var id int
	var createdAt, updatedAt time.Time

	err = r.db.QueryRowContext(
		ctx,
		query,
		req.GoogleID,
		req.Email,
		req.Name,
		req.ProfilePictureURL,
		encryptedAccessToken,
		encryptedRefreshToken,
		req.GoogleTokenExpiry,
		now,
		now,
		now,
	).Scan(&id, &createdAt, &updatedAt)

	if err != nil {
		return nil, err
	}

	// Build the user object
	user = User{
		ID:                        id,
		GoogleID:                  req.GoogleID,
		Email:                     req.Email,
		Name:                      req.Name,
		ProfilePictureURL:         req.ProfilePictureURL,
		GoogleAccessToken:         encryptedAccessToken,
		GoogleRefreshToken:        encryptedRefreshToken,
		GoogleTokenExpiry:         req.GoogleTokenExpiry,
		Timezone:                  "UTC", // Default timezone
		EmailNotificationsEnabled: true,  // Default enabled
		AutomationEnabled:         false, // Default disabled
		CreatedAt:                 createdAt,
		UpdatedAt:                 updatedAt,
		LastLoginAt:               &now,
	}

	return &user, nil
}

// GetUserByGoogleID retrieves a user by their Google ID
func (r *UserRepository) GetUserByGoogleID(ctx context.Context, googleID string) (*User, error) {
	query := `
		SELECT id, google_id, email, name, profile_picture_url,
			   google_access_token, google_refresh_token, google_token_expiry,
			   strava_access_token, strava_refresh_token, strava_token_expiry, strava_athlete_id,
			   strava_athlete_name, strava_profile_picture_url,
			   spreadsheet_id, timezone, email_notifications_enabled, automation_enabled,
			   created_at, updated_at, last_login_at
		FROM users WHERE google_id = $1
	`

	var user User
	err := r.db.QueryRowContext(ctx, query, googleID).Scan(
		&user.ID, &user.GoogleID, &user.Email, &user.Name, &user.ProfilePictureURL,
		&user.GoogleAccessToken, &user.GoogleRefreshToken, &user.GoogleTokenExpiry,
		&user.StravaAccessToken, &user.StravaRefreshToken, &user.StravaTokenExpiry, &user.StravaAthleteID,
		&user.StravaAthleteName, &user.StravaProfilePictureURL,
		&user.SpreadsheetID, &user.Timezone, &user.EmailNotificationsEnabled, &user.AutomationEnabled,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, err
	}

	return &user, nil
}

// GetUserByID retrieves a user by their ID
func (r *UserRepository) GetUserByID(ctx context.Context, id int) (*User, error) {
	query := `
		SELECT id, google_id, email, name, profile_picture_url,
			   google_access_token, google_refresh_token, google_token_expiry,
			   strava_access_token, strava_refresh_token, strava_token_expiry, strava_athlete_id,
			   strava_athlete_name, strava_profile_picture_url,
			   spreadsheet_id, timezone, email_notifications_enabled, automation_enabled,
			   created_at, updated_at, last_login_at
		FROM users WHERE id = $1
	`

	var user User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.GoogleID, &user.Email, &user.Name, &user.ProfilePictureURL,
		&user.GoogleAccessToken, &user.GoogleRefreshToken, &user.GoogleTokenExpiry,
		&user.StravaAccessToken, &user.StravaRefreshToken, &user.StravaTokenExpiry, &user.StravaAthleteID,
		&user.StravaAthleteName, &user.StravaProfilePictureURL,
		&user.SpreadsheetID, &user.Timezone, &user.EmailNotificationsEnabled, &user.AutomationEnabled,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, err
	}

	return &user, nil
}

// UpdateUserTokens updates a user's Google OAuth tokens and optionally last login timestamp
func (r *UserRepository) UpdateUserTokens(ctx context.Context, req *UpdateUserTokensRequest) error {
	// Encrypt OAuth tokens
	encryptedAccessToken, err := r.encryptor.Encrypt(req.GoogleAccessToken)
	if err != nil {
		return err
	}

	encryptedRefreshToken, err := r.encryptor.Encrypt(req.GoogleRefreshToken)
	if err != nil {
		return err
	}

	now := time.Now()
	
	// Build query based on whether we should update last login
	var query string
	var args []interface{}
	
	if req.UpdateLastLogin {
		query = `
			UPDATE users 
			SET google_access_token = $1,
				google_refresh_token = $2,
				google_token_expiry = $3,
				last_login_at = $4,
				updated_at = $5
			WHERE id = $6
		`
		args = []interface{}{
			encryptedAccessToken,
			encryptedRefreshToken,
			req.GoogleTokenExpiry,
			now,
			now,
			req.UserID,
		}
	} else {
		query = `
			UPDATE users 
			SET google_access_token = $1,
				google_refresh_token = $2,
				google_token_expiry = $3,
				updated_at = $4
			WHERE id = $5
		`
		args = []interface{}{
			encryptedAccessToken,
			encryptedRefreshToken,
			req.GoogleTokenExpiry,
			now,
			req.UserID,
		}
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	
	// Check that exactly one row was affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		return sql.ErrNoRows // No user found with the given ID
	}
	
	return nil
}

// UpdateLastLoginAt updates the user's last login timestamp
func (r *UserRepository) UpdateLastLoginAt(ctx context.Context, userID int) error {
	query := `UPDATE users SET last_login_at = $1, updated_at = $2 WHERE id = $3`
	result, err := r.db.ExecContext(ctx, query, time.Now(), time.Now(), userID)
	if err != nil {
		return err
	}
	
	// Check that exactly one row was affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		return sql.ErrNoRows // No user found with the given ID
	}
	
	return nil
}

// GetDecryptedGoogleTokens retrieves and decrypts a user's Google OAuth tokens
func (r *UserRepository) GetDecryptedGoogleTokens(ctx context.Context, userID int) (accessToken, refreshToken string, expiry *time.Time, err error) {
	query := `
		SELECT google_access_token, google_refresh_token, google_token_expiry
		FROM users WHERE id = $1
	`

	var encryptedAccessToken, encryptedRefreshToken []byte
	err = r.db.QueryRowContext(ctx, query, userID).Scan(&encryptedAccessToken, &encryptedRefreshToken, &expiry)
	if err != nil {
		return "", "", nil, err
	}

	// Decrypt tokens
	accessToken, err = r.encryptor.Decrypt(encryptedAccessToken)
	if err != nil {
		return "", "", nil, err
	}

	refreshToken, err = r.encryptor.Decrypt(encryptedRefreshToken)
	if err != nil {
		return "", "", nil, err
	}

	return accessToken, refreshToken, expiry, nil
}

// DecryptToken decrypts an encrypted token
func (r *UserRepository) DecryptToken(encryptedToken []byte) (string, error) {
	if len(encryptedToken) == 0 {
		return "", nil // No token to decrypt
	}
	
	return r.encryptor.Decrypt(encryptedToken)
}

// UpdateStravaConnection updates the user's Strava connection with encrypted tokens and profile information
func (r *UserRepository) UpdateStravaConnection(ctx context.Context, userID int, accessToken, refreshToken string, expiry *time.Time, athleteID int64, athleteName, profilePictureURL string) error {
	// Encrypt Strava tokens
	encryptedAccessToken, err := r.encryptor.Encrypt(accessToken)
	if err != nil {
		return err
	}

	encryptedRefreshToken, err := r.encryptor.Encrypt(refreshToken)
	if err != nil {
		return err
	}

	query := `
		UPDATE users 
		SET strava_access_token = $1, 
		    strava_refresh_token = $2, 
		    strava_token_expiry = $3, 
		    strava_athlete_id = $4, 
		    strava_athlete_name = $5,
		    strava_profile_picture_url = $6,
		    updated_at = $7 
		WHERE id = $8
	`

	now := time.Now()
	_, err = r.db.ExecContext(ctx, query, 
		encryptedAccessToken, 
		encryptedRefreshToken, 
		expiry, 
		athleteID, 
		athleteName,
		profilePictureURL,
		now, 
		userID)
	
	return err
}

// RemoveStravaConnection removes the user's Strava connection by clearing tokens and athlete ID
func (r *UserRepository) RemoveStravaConnection(ctx context.Context, userID int) error {
	query := `
		UPDATE users 
		SET strava_access_token = NULL, 
		    strava_refresh_token = NULL, 
		    strava_token_expiry = NULL, 
		    strava_athlete_id = NULL, 
		    strava_athlete_name = NULL,
		    strava_profile_picture_url = NULL,
		    updated_at = $1 
		WHERE id = $2
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, now, userID)
	return err
}

// GetDecryptedStravaTokens retrieves and decrypts a user's Strava OAuth tokens
func (r *UserRepository) GetDecryptedStravaTokens(ctx context.Context, userID int) (accessToken, refreshToken string, expiry *time.Time, athleteID *int64, err error) {
	query := `
		SELECT strava_access_token, strava_refresh_token, strava_token_expiry, strava_athlete_id
		FROM users WHERE id = $1
	`

	var encryptedAccessToken, encryptedRefreshToken []byte
	err = r.db.QueryRowContext(ctx, query, userID).Scan(&encryptedAccessToken, &encryptedRefreshToken, &expiry, &athleteID)
	if err != nil {
		return "", "", nil, nil, err
	}

	// Handle case where user has no Strava connection
	if len(encryptedAccessToken) == 0 || len(encryptedRefreshToken) == 0 {
		return "", "", nil, athleteID, nil
	}

	// Decrypt tokens
	accessToken, err = r.encryptor.Decrypt(encryptedAccessToken)
	if err != nil {
		return "", "", nil, nil, err
	}

	refreshToken, err = r.encryptor.Decrypt(encryptedRefreshToken)
	if err != nil {
		return "", "", nil, nil, err
	}

	return accessToken, refreshToken, expiry, athleteID, nil
}

// UpdateSpreadsheetID updates the user's Google Spreadsheet ID
func (r *UserRepository) UpdateSpreadsheetID(ctx context.Context, userID int, spreadsheetID string) error {
	query := `
		UPDATE users 
		SET spreadsheet_id = $1, updated_at = $2 
		WHERE id = $3
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, spreadsheetID, now, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// ClearSpreadsheetID clears the user's Google Spreadsheet ID
func (r *UserRepository) ClearSpreadsheetID(ctx context.Context, userID int) error {
	query := `
		UPDATE users 
		SET spreadsheet_id = NULL, updated_at = $1 
		WHERE id = $2
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, now, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GetProcessingConfigForUser retrieves all necessary data for automation processing for a specific user
// This method is optimized for the automation engine and fetches all required fields in a single query.
// It returns decrypted tokens ready for use by API clients.
func (r *UserRepository) GetProcessingConfigForUser(ctx context.Context, userID int) (*ProcessingTokens, error) {
	query := `
		SELECT google_access_token, google_refresh_token, google_token_expiry,
			   strava_access_token, strava_refresh_token, strava_token_expiry, strava_athlete_id,
			   spreadsheet_id, COALESCE(timezone, ''), COALESCE(email, '')
		FROM users WHERE id = $1
	`

	var encryptedGoogleAccessToken, encryptedGoogleRefreshToken []byte
	var encryptedStravaAccessToken, encryptedStravaRefreshToken []byte
	var googleExpiry, stravaExpiry *time.Time
	var athleteID *int64
	var spreadsheetID *string
	var timezone, email string

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&encryptedGoogleAccessToken, &encryptedGoogleRefreshToken, &googleExpiry,
		&encryptedStravaAccessToken, &encryptedStravaRefreshToken, &stravaExpiry, &athleteID,
		&spreadsheetID, &timezone, &email,
	)

	if err != nil {
		return nil, err
	}

	result := &ProcessingTokens{
		GoogleTokenExpiry: googleExpiry,
		StravaTokenExpiry: stravaExpiry,
		StravaAthleteID:   athleteID,
		SpreadsheetID:     spreadsheetID,
		Timezone:          timezone,
		Email:             email,
	}

	// Decrypt Google tokens
	if len(encryptedGoogleAccessToken) > 0 {
		result.GoogleAccessToken, err = r.encryptor.Decrypt(encryptedGoogleAccessToken)
		if err != nil {
			return nil, err
		}
	}

	if len(encryptedGoogleRefreshToken) > 0 {
		result.GoogleRefreshToken, err = r.encryptor.Decrypt(encryptedGoogleRefreshToken)
		if err != nil {
			return nil, err
		}
	}

	// Decrypt Strava tokens
	if len(encryptedStravaAccessToken) > 0 {
		result.StravaAccessToken, err = r.encryptor.Decrypt(encryptedStravaAccessToken)
		if err != nil {
			return nil, err
		}
	}

	if len(encryptedStravaRefreshToken) > 0 {
		result.StravaRefreshToken, err = r.encryptor.Decrypt(encryptedStravaRefreshToken)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}