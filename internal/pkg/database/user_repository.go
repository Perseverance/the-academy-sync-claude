package database

import (
	"database/sql"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
)

// UserRepository handles database operations for users
type UserRepository struct {
	db        *sql.DB
	encryptor *auth.EncryptionService
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sql.DB, encryptor *auth.EncryptionService) *UserRepository {
	return &UserRepository{
		db:        db,
		encryptor: encryptor,
	}
}

// CreateUser creates a new user in the database
func (r *UserRepository) CreateUser(req *CreateUserRequest) (*User, error) {
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

	err = r.db.QueryRow(
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
func (r *UserRepository) GetUserByGoogleID(googleID string) (*User, error) {
	query := `
		SELECT id, google_id, email, name, profile_picture_url,
			   google_access_token, google_refresh_token, google_token_expiry,
			   strava_access_token, strava_refresh_token, strava_token_expiry, strava_athlete_id,
			   spreadsheet_id, timezone, email_notifications_enabled, automation_enabled,
			   created_at, updated_at, last_login_at
		FROM users WHERE google_id = $1
	`

	var user User
	err := r.db.QueryRow(query, googleID).Scan(
		&user.ID, &user.GoogleID, &user.Email, &user.Name, &user.ProfilePictureURL,
		&user.GoogleAccessToken, &user.GoogleRefreshToken, &user.GoogleTokenExpiry,
		&user.StravaAccessToken, &user.StravaRefreshToken, &user.StravaTokenExpiry, &user.StravaAthleteID,
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
func (r *UserRepository) GetUserByID(id int) (*User, error) {
	query := `
		SELECT id, google_id, email, name, profile_picture_url,
			   google_access_token, google_refresh_token, google_token_expiry,
			   strava_access_token, strava_refresh_token, strava_token_expiry, strava_athlete_id,
			   spreadsheet_id, timezone, email_notifications_enabled, automation_enabled,
			   created_at, updated_at, last_login_at
		FROM users WHERE id = $1
	`

	var user User
	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.GoogleID, &user.Email, &user.Name, &user.ProfilePictureURL,
		&user.GoogleAccessToken, &user.GoogleRefreshToken, &user.GoogleTokenExpiry,
		&user.StravaAccessToken, &user.StravaRefreshToken, &user.StravaTokenExpiry, &user.StravaAthleteID,
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

// UpdateUserTokens updates a user's Google OAuth tokens
func (r *UserRepository) UpdateUserTokens(req *UpdateUserTokensRequest) error {
	// Encrypt OAuth tokens
	encryptedAccessToken, err := r.encryptor.Encrypt(req.GoogleAccessToken)
	if err != nil {
		return err
	}

	encryptedRefreshToken, err := r.encryptor.Encrypt(req.GoogleRefreshToken)
	if err != nil {
		return err
	}

	query := `
		UPDATE users 
		SET google_access_token = $1,
			google_refresh_token = $2,
			google_token_expiry = $3,
			updated_at = $4
		WHERE id = $5
	`

	_, err = r.db.Exec(
		query,
		encryptedAccessToken,
		encryptedRefreshToken,
		req.GoogleTokenExpiry,
		time.Now(),
		req.UserID,
	)

	return err
}

// UpdateLastLoginAt updates the user's last login timestamp
func (r *UserRepository) UpdateLastLoginAt(userID int) error {
	query := `UPDATE users SET last_login_at = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.Exec(query, time.Now(), time.Now(), userID)
	return err
}

// GetDecryptedGoogleTokens retrieves and decrypts a user's Google OAuth tokens
func (r *UserRepository) GetDecryptedGoogleTokens(userID int) (accessToken, refreshToken string, expiry *time.Time, err error) {
	query := `
		SELECT google_access_token, google_refresh_token, google_token_expiry
		FROM users WHERE id = $1
	`

	var encryptedAccessToken, encryptedRefreshToken []byte
	err = r.db.QueryRow(query, userID).Scan(&encryptedAccessToken, &encryptedRefreshToken, &expiry)
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