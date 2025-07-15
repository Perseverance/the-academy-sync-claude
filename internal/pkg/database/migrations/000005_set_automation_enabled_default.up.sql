-- Update existing users to have automation enabled by default
UPDATE users SET automation_enabled = true WHERE automation_enabled = false;

-- Change the default value for automation_enabled to true for new users
ALTER TABLE users ALTER COLUMN automation_enabled SET DEFAULT true;