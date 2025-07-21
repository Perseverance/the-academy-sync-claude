-- Remove reauth required flags from users table

-- Drop indexes first
DROP INDEX IF EXISTS idx_users_google_reauth_required;
DROP INDEX IF EXISTS idx_users_strava_reauth_required;

-- Remove columns
ALTER TABLE users 
DROP COLUMN IF EXISTS google_reauth_required,
DROP COLUMN IF EXISTS strava_reauth_required;