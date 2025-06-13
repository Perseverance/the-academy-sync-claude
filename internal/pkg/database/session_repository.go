package database

import (
	"database/sql"
	"time"
)

// SessionRepository handles database operations for user sessions
type SessionRepository struct {
	db *sql.DB
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// CreateSession creates a new user session in the database
func (r *SessionRepository) CreateSession(req *CreateSessionRequest) (*UserSession, error) {
	query := `
		INSERT INTO user_sessions (
			user_id, session_token, user_agent, ip_address, 
			created_at, expires_at, last_used_at, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, last_used_at
	`

	now := time.Now()
	var session UserSession
	var id int
	var createdAt, lastUsedAt time.Time

	err := r.db.QueryRow(
		query,
		req.UserID,
		req.SessionToken,
		req.UserAgent,
		req.IPAddress,
		now,
		req.ExpiresAt,
		now,
		true, // is_active
	).Scan(&id, &createdAt, &lastUsedAt)

	if err != nil {
		return nil, err
	}

	session = UserSession{
		ID:           id,
		UserID:       req.UserID,
		SessionToken: req.SessionToken,
		UserAgent:    req.UserAgent,
		IPAddress:    req.IPAddress,
		CreatedAt:    createdAt,
		ExpiresAt:    req.ExpiresAt,
		LastUsedAt:   lastUsedAt,
		IsActive:     true,
	}

	return &session, nil
}

// GetSessionByToken retrieves a session by its token
func (r *SessionRepository) GetSessionByToken(token string) (*UserSession, error) {
	query := `
		SELECT id, user_id, session_token, user_agent, ip_address,
			   created_at, expires_at, last_used_at, is_active
		FROM user_sessions 
		WHERE session_token = $1 AND is_active = true AND expires_at > $2
	`

	var session UserSession
	err := r.db.QueryRow(query, token, time.Now()).Scan(
		&session.ID, &session.UserID, &session.SessionToken,
		&session.UserAgent, &session.IPAddress,
		&session.CreatedAt, &session.ExpiresAt, &session.LastUsedAt, &session.IsActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Session not found or expired
		}
		return nil, err
	}

	return &session, nil
}

// UpdateSessionLastUsed updates the last used timestamp for a session
func (r *SessionRepository) UpdateSessionLastUsed(sessionID int) error {
	query := `UPDATE user_sessions SET last_used_at = $1 WHERE id = $2`
	_, err := r.db.Exec(query, time.Now(), sessionID)
	return err
}

// DeactivateSession marks a session as inactive (logout)
func (r *SessionRepository) DeactivateSession(sessionID int) error {
	query := `UPDATE user_sessions SET is_active = false WHERE id = $1`
	_, err := r.db.Exec(query, sessionID)
	return err
}

// DeactivateAllUserSessions marks all sessions for a user as inactive
func (r *SessionRepository) DeactivateAllUserSessions(userID int) error {
	query := `UPDATE user_sessions SET is_active = false WHERE user_id = $1`
	_, err := r.db.Exec(query, userID)
	return err
}

// CleanupExpiredSessions removes expired sessions from the database
func (r *SessionRepository) CleanupExpiredSessions() error {
	query := `DELETE FROM user_sessions WHERE expires_at < $1`
	_, err := r.db.Exec(query, time.Now())
	return err
}

// GetUserActiveSessions retrieves all active sessions for a user
func (r *SessionRepository) GetUserActiveSessions(userID int) ([]*UserSession, error) {
	query := `
		SELECT id, user_id, session_token, user_agent, ip_address,
			   created_at, expires_at, last_used_at, is_active
		FROM user_sessions 
		WHERE user_id = $1 AND is_active = true AND expires_at > $2
		ORDER BY last_used_at DESC
	`

	rows, err := r.db.Query(query, userID, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*UserSession
	for rows.Next() {
		var session UserSession
		err := rows.Scan(
			&session.ID, &session.UserID, &session.SessionToken,
			&session.UserAgent, &session.IPAddress,
			&session.CreatedAt, &session.ExpiresAt, &session.LastUsedAt, &session.IsActive,
		)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}