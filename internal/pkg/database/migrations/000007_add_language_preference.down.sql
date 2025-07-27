-- Remove language preference column from users table

-- Drop index first
DROP INDEX IF EXISTS idx_users_language_preference;

-- Remove column
ALTER TABLE users 
DROP COLUMN IF EXISTS language_preference;