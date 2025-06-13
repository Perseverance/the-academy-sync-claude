-- Create users table to store user profile information and encrypted OAuth tokens
CREATE TABLE users (
    id SERIAL PRIMARY KEY,                                    -- Auto-incrementing primary key
    google_id VARCHAR(255) NOT NULL UNIQUE,                   -- Google OAuth user ID (unique identifier from Google)
    email VARCHAR(255) NOT NULL UNIQUE,                       -- User's email address (unique)
    name VARCHAR(255) NOT NULL,                               -- User's display name from Google profile
    profile_picture_url TEXT,                                 -- URL to user's Google profile picture
    
    -- Encrypted OAuth tokens stored as binary data
    google_access_token BYTEA,                                -- Encrypted Google OAuth access token
    google_refresh_token BYTEA,                               -- Encrypted Google OAuth refresh token
    google_token_expiry TIMESTAMPTZ,                          -- Expiry time for Google access token
    
    strava_access_token BYTEA,                                -- Encrypted Strava OAuth access token
    strava_refresh_token BYTEA,                               -- Encrypted Strava OAuth refresh token
    strava_token_expiry TIMESTAMPTZ,                          -- Expiry time for Strava access token
    strava_athlete_id BIGINT,                                 -- Strava athlete ID
    
    -- User configuration and preferences
    spreadsheet_id VARCHAR(255),                              -- Google Sheets spreadsheet ID for automation
    timezone VARCHAR(50) DEFAULT 'UTC',                       -- User's timezone preference
    
    -- Feature flags and preferences
    email_notifications_enabled BOOLEAN DEFAULT true,         -- Whether user wants email notifications
    automation_enabled BOOLEAN DEFAULT false,                 -- Whether automation is active for this user
    
    -- Audit timestamps
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,         -- When user record was created
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,         -- When user record was last updated
    last_login_at TIMESTAMPTZ                                 -- When user last logged in
);

-- Create indexes for efficient queries
CREATE INDEX idx_users_email ON users(email);                -- Index on email for login queries
CREATE INDEX idx_users_google_id ON users(google_id);        -- Index on Google ID for OAuth flows

-- Create user_sessions table to manage user authentication sessions
CREATE TABLE user_sessions (
    id SERIAL PRIMARY KEY,                                    -- Auto-incrementing primary key
    user_id INTEGER NOT NULL,                                 -- Foreign key to users table
    session_token VARCHAR(255) NOT NULL UNIQUE,               -- Unique session token (JWT or similar)
    
    -- Session metadata
    user_agent TEXT,                                          -- Browser/client user agent string
    ip_address INET,                                          -- Client IP address
    
    -- Session timing
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,         -- When session was created
    expires_at TIMESTAMPTZ NOT NULL,                          -- When session expires
    last_used_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,       -- When session was last used
    
    -- Session state
    is_active BOOLEAN DEFAULT true,                           -- Whether session is currently active
    
    -- Foreign key constraint with cascade delete
    CONSTRAINT fk_user_sessions_user_id 
        FOREIGN KEY (user_id) 
        REFERENCES users(id) 
        ON DELETE CASCADE
);

-- Create indexes for efficient session queries
CREATE INDEX idx_user_sessions_token ON user_sessions(session_token);     -- Index on session token for auth
CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id);         -- Index on user_id for user queries
CREATE INDEX idx_user_sessions_expires_at ON user_sessions(expires_at);   -- Index on expiry for cleanup queries