-- Increase session_token column length to support JWT tokens
-- JWT tokens can be 500-1000+ characters, so we'll use TEXT for unlimited length
ALTER TABLE user_sessions ALTER COLUMN session_token TYPE TEXT;