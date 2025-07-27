-- Add language preference column to users table
-- This column stores the user's preferred language for UI and email notifications
-- Default is set to 'bg' (Bulgarian) as per requirements

ALTER TABLE users 
ADD COLUMN language_preference VARCHAR(10) DEFAULT 'bg' NOT NULL;

-- Create index for efficient queries when filtering by language
CREATE INDEX idx_users_language_preference ON users(language_preference);

-- Add comment for documentation
COMMENT ON COLUMN users.language_preference IS 'User''s preferred language for UI and email notifications (ISO 639-1 code)';