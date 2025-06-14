# Database Migrations

This directory contains database migrations for the Academy Sync application.

## Migration 000002: Session Token Length

### Purpose
Increases the `session_token` column length from `VARCHAR(255)` to `TEXT` to support JWT tokens, which can be 500-1000+ characters long.

### Files
- `000002_increase_session_token_length.up.sql` - Forward migration (safe)
- `000002_increase_session_token_length.down.sql` - Reverse migration (with safeguards)
- `000002_cleanup_long_tokens.sql` - Helper script for cleanup

### Forward Migration (Up)
The up migration is **safe** and can be run without concerns:
```sql
ALTER TABLE user_sessions ALTER COLUMN session_token TYPE TEXT;
```

This only expands the column capacity and doesn't risk data loss.

### Reverse Migration (Down)
The down migration includes **safety checks** to prevent data loss:

1. **Validation Phase**: Checks if any session tokens exceed 255 characters
2. **Abort on Risk**: Raises an exception if long tokens exist
3. **Safe Execution**: Only proceeds if all tokens fit in VARCHAR(255)
4. **Explicit Casting**: Uses `USING` clause for safe type conversion

### If Down Migration Fails

If the down migration fails due to long session tokens, you have several options:

#### Option 1: View Affected Sessions (Recommended First)
```sql
-- Run this to see what sessions would be affected
\i 000002_cleanup_long_tokens.sql
```

#### Option 2: Clean Up Expired Sessions (Safest)
```sql
DELETE FROM user_sessions 
WHERE LENGTH(session_token) > 255 
AND expires_at < NOW();
```

#### Option 3: Delete All Long Tokens (Maintenance Window)
```sql
-- WARNING: This will log out users with long session tokens
DELETE FROM user_sessions WHERE LENGTH(session_token) > 255;
```

#### Option 4: Wait for Natural Expiry
JWT tokens expire after 24 hours, so you can wait for long tokens to naturally expire before running the down migration.

### Best Practices

1. **Always test migrations** on a copy of production data first
2. **Run during maintenance windows** if user logout is acceptable
3. **Check affected sessions** before cleaning up
4. **Prefer cleaning expired sessions** over active ones
5. **Consider user impact** of forced logouts

### Error Messages

If the down migration fails, you'll see:
```
ERROR: Cannot revert session_token to VARCHAR(255): X session tokens exceed 255 characters (max length: Y). 
Please clean up long session tokens before running this migration.
```

This is by design to protect your data!