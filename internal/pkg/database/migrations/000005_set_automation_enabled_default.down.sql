-- Revert the default value for automation_enabled back to false
ALTER TABLE users ALTER COLUMN automation_enabled SET DEFAULT false;

-- Note: We don't revert existing users back to false as that would be a destructive operation