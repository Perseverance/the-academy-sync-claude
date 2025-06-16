-- Remove Strava profile fields from users table
ALTER TABLE users 
DROP COLUMN strava_athlete_name,
DROP COLUMN strava_profile_picture_url;