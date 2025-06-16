-- Add Strava profile fields to users table
ALTER TABLE users 
ADD COLUMN strava_athlete_name VARCHAR(255),
ADD COLUMN strava_profile_picture_url VARCHAR(500);

-- Add comment explaining the fields
COMMENT ON COLUMN users.strava_athlete_name IS 'Full name from Strava athlete profile';
COMMENT ON COLUMN users.strava_profile_picture_url IS 'Profile picture URL from Strava athlete profile';