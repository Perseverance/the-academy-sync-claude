-- Add reauth required flags to users table
-- These flags are set when authentication failures occur during processing
-- and indicate that the user needs to re-authorize the respective service

ALTER TABLE users 
ADD COLUMN google_reauth_required BOOLEAN DEFAULT false,
ADD COLUMN strava_reauth_required BOOLEAN DEFAULT false;

-- Create indexes for efficient queries on reauth flags
CREATE INDEX idx_users_google_reauth_required ON users(google_reauth_required) WHERE google_reauth_required = true;
CREATE INDEX idx_users_strava_reauth_required ON users(strava_reauth_required) WHERE strava_reauth_required = true;

-- Add comments for documentation
COMMENT ON COLUMN users.google_reauth_required IS 'Flag indicating whether user needs to re-authorize Google OAuth access';
COMMENT ON COLUMN users.strava_reauth_required IS 'Flag indicating whether user needs to re-authorize Strava OAuth access';