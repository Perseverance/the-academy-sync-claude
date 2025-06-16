-- Test script to demonstrate migration safeguards
-- This script shows how the down migration behaves with different data scenarios

-- Scenario 1: Safe case - all tokens <= 255 characters
-- CREATE TEMPORARY TABLE test_user_sessions_safe AS
-- SELECT 1 as id, 1 as user_id, REPEAT('a', 200) as session_token, NOW() as created_at, NOW() + INTERVAL '1 day' as expires_at;

-- Test the validation logic for safe case:
-- DO $$
-- DECLARE
--     long_token_count INTEGER;
--     max_token_length INTEGER;
-- BEGIN
--     SELECT COUNT(*), COALESCE(MAX(LENGTH(session_token)), 0)
--     INTO long_token_count, max_token_length
--     FROM test_user_sessions_safe 
--     WHERE LENGTH(session_token) > 255;
--     
--     RAISE NOTICE 'Safe case: % long tokens, max length: %', long_token_count, max_token_length;
-- END $$;

-- Scenario 2: Unsafe case - tokens > 255 characters
-- CREATE TEMPORARY TABLE test_user_sessions_unsafe AS
-- SELECT 1 as id, 1 as user_id, REPEAT('b', 500) as session_token, NOW() as created_at, NOW() + INTERVAL '1 day' as expires_at;

-- Test the validation logic for unsafe case:
-- DO $$
-- DECLARE
--     long_token_count INTEGER;
--     max_token_length INTEGER;
-- BEGIN
--     SELECT COUNT(*), COALESCE(MAX(LENGTH(session_token)), 0)
--     INTO long_token_count, max_token_length
--     FROM test_user_sessions_unsafe 
--     WHERE LENGTH(session_token) > 255;
--     
--     IF long_token_count > 0 THEN
--         RAISE NOTICE 'Unsafe case detected: % tokens exceed 255 chars (max: %)', long_token_count, max_token_length;
--         -- In real migration, this would be RAISE EXCEPTION
--     END IF;
-- END $$;

-- Example of the actual validation logic used in the migration:
SELECT 
    'Validation example' as test_type,
    COUNT(*) as long_token_count,
    COALESCE(MAX(LENGTH('this-is-a-sample-jwt-token-that-could-be-very-long-' || REPEAT('x', 300))), 0) as max_length,
    CASE 
        WHEN COUNT(*) > 0 THEN 'WOULD FAIL MIGRATION'
        ELSE 'MIGRATION SAFE'
    END as migration_status
FROM (SELECT 'this-is-a-sample-jwt-token-that-could-be-very-long-' || REPEAT('x', 300) as token) t
WHERE LENGTH(token) > 255;