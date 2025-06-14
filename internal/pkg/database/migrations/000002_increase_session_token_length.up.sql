-- Increase session_token column length to support JWT tokens
-- JWT tokens can be 500-1000+ characters, so we'll use TEXT for unlimited length
-- This migration is safe as it only expands the column capacity
ALTER TABLE user_sessions ALTER COLUMN session_token TYPE TEXT;