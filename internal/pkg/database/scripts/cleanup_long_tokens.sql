-- Helper script to clean up session tokens longer than 255 characters
-- Run this BEFORE attempting the down migration if it fails due to long tokens

-- Option 1: Delete sessions with tokens longer than 255 characters
-- WARNING: This will log out users with long session tokens
-- DELETE FROM user_sessions WHERE LENGTH(session_token) > 255;

-- Option 2: Truncate session tokens to 255 characters (NOT RECOMMENDED)
-- WARNING: This will invalidate the truncated tokens, effectively logging out users
-- UPDATE user_sessions 
-- SET session_token = LEFT(session_token, 255) 
-- WHERE LENGTH(session_token) > 255;

-- Option 3: View sessions that would be affected (RECOMMENDED to run first)
SELECT 
    id,
    user_id,
    LENGTH(session_token) as token_length,
    created_at,
    expires_at,
    CASE 
        WHEN expires_at < NOW() THEN 'EXPIRED'
        ELSE 'ACTIVE'
    END as status
FROM user_sessions 
WHERE LENGTH(session_token) > 255
ORDER BY LENGTH(session_token) DESC;

-- Option 4: Delete only expired sessions with long tokens (SAFEST)
-- This preserves active user sessions while cleaning up expired ones
-- DELETE FROM user_sessions 
-- WHERE LENGTH(session_token) > 255 
-- AND expires_at < NOW();

-- Recommended approach:
-- 1. Run Option 3 to see what would be affected
-- 2. Run Option 4 to clean up expired sessions safely
-- 3. If active sessions remain, consider Option 1 during maintenance window
-- 4. Then run the down migration