-- Revert session_token column back to VARCHAR(255)
-- Note: This may fail if there are tokens longer than 255 characters
ALTER TABLE user_sessions ALTER COLUMN session_token TYPE VARCHAR(255);