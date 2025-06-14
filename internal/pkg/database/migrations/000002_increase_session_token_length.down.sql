-- Revert session_token column back to VARCHAR(255)
-- This migration includes safeguards to prevent data loss

-- First, check if any session_token exceeds 255 characters
-- This will abort the migration if any tokens would be truncated
DO $$
DECLARE
    long_token_count INTEGER;
    max_token_length INTEGER;
BEGIN
    -- Count session tokens longer than 255 characters
    SELECT COUNT(*), COALESCE(MAX(LENGTH(session_token)), 0)
    INTO long_token_count, max_token_length
    FROM user_sessions 
    WHERE LENGTH(session_token) > 255;
    
    -- If any tokens exceed 255 characters, abort the migration
    IF long_token_count > 0 THEN
        RAISE EXCEPTION 
            'Cannot revert session_token to VARCHAR(255): % session tokens exceed 255 characters (max length: %). ' ||
            'Please clean up long session tokens before running this migration.',
            long_token_count, max_token_length;
    END IF;
    
    -- Log successful validation
    RAISE NOTICE 'Migration validation passed: All session tokens are <= 255 characters';
END $$;

-- Safe revert with explicit casting
ALTER TABLE user_sessions 
ALTER COLUMN session_token TYPE VARCHAR(255) 
USING session_token::VARCHAR(255);