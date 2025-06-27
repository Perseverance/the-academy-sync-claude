# Deployment Scripts

This directory contains helper scripts for managing secrets and database migrations for The Academy Sync.

## Prerequisites

1. **Google Cloud SDK** installed and authenticated:
   ```bash
   gcloud auth login
   gcloud config set project the-academy-sync-sdlc-test
   ```

2. **Terraform** installed and initialized in the `terraform/` directory

3. **golang-migrate** installed for database migrations:
   ```bash
   brew install golang-migrate
   ```

4. **Python 3** for URL encoding (usually pre-installed on macOS)

## Scripts

### 1. manage-secrets.sh

Manages secrets in Google Secret Manager based on environment files.

**Usage:**
```bash
./scripts/manage-secrets.sh [create|view|update] [staging|prod]
```

**Commands:**
- `create` - Create new secrets from .env file (skips existing secrets)
- `update` - Update existing secrets or create new ones
- `view` - List all secrets for the environment

**Examples:**
```bash
# First-time setup for staging
cp .env.staging.example .env.staging
# Edit .env.staging with your values
./scripts/manage-secrets.sh create staging

# Update existing secrets
./scripts/manage-secrets.sh update staging

# View all secrets (without values)
./scripts/manage-secrets.sh view staging

# View secrets with values
VERBOSE=true ./scripts/manage-secrets.sh view staging
```

**Features:**
- Reads configuration from `.env.staging` or `.env.prod`
- Automatically constructs database URL from Terraform outputs
- Automatically constructs Redis URL from Terraform outputs (when Memorystore is deployed)
- Handles URL encoding for special characters in passwords
- Maps environment variables to secret names

### 2. migrate-db.sh

Runs database migrations on staging or production databases.

**Usage:**
```bash
./scripts/migrate-db.sh [staging|prod] [options]
```

**Options:**
- `--proxy` - Use Cloud SQL Proxy for secure connection (recommended)
- `--down N` - Rollback N migrations
- `--force VERSION` - Force database to specific version
- `--status` - Show current migration version

**Examples:**
```bash
# Run all pending migrations on staging
./scripts/migrate-db.sh staging

# Run migrations on production using Cloud SQL Proxy
./scripts/migrate-db.sh prod --proxy

# Check current migration status
./scripts/migrate-db.sh staging --status

# Rollback last migration
./scripts/migrate-db.sh staging --down 1

# Force to specific version (use with caution!)
./scripts/migrate-db.sh staging --force 3
```

**Features:**
- Automatically fetches database connection details from Terraform
- Retrieves password from Secret Manager
- Handles URL encoding for special characters
- Supports both direct connection and Cloud SQL Proxy
- Shows migration status after operations

## Workflow

### Initial Setup

1. **Deploy infrastructure with Terraform:**
   ```bash
   cd terraform
   terraform workspace select staging
   terraform apply -var-file="staging.tfvars"
   ```

2. **Create environment file:**
   ```bash
   cp .env.staging.example .env.staging
   # Edit .env.staging with your actual values
   ```

3. **Create secrets:**
   ```bash
   ./scripts/manage-secrets.sh create staging
   ```

4. **Run database migrations:**
   ```bash
   ./scripts/migrate-db.sh staging --proxy
   ```

### Updating Secrets

When you need to update secrets (e.g., rotating keys):

1. Update the values in `.env.staging` or `.env.prod`
2. Run the update command:
   ```bash
   ./scripts/manage-secrets.sh update staging
   ```

### Production Deployment

1. **Deploy infrastructure:**
   ```bash
   cd terraform
   terraform workspace select prod
   terraform apply -var-file="prod.tfvars"
   ```

2. **Setup production secrets:**
   ```bash
   cp .env.prod.example .env.prod
   # Edit .env.prod carefully
   ./scripts/manage-secrets.sh create prod
   ```

3. **Run migrations with proxy:**
   ```bash
   ./scripts/migrate-db.sh prod --proxy
   ```

## Security Notes

1. **Never commit** `.env.staging` or `.env.prod` files
2. Use `--proxy` for production database connections
3. Store OAuth credentials securely
4. Rotate secrets regularly, especially JWT and encryption secrets
5. Limit access to Secret Manager using IAM policies

## Troubleshooting

### Database Connection Issues

1. **"invalid port" error**: Password contains special characters that need URL encoding. The script handles this automatically.

2. **Connection timeout**: Check authorized networks in Terraform configuration or use `--proxy`

3. **Permission denied**: Ensure your Google account has Secret Manager access

### Secret Manager Issues

1. **"Permission denied"**: Run `gcloud auth login` and ensure you have the Secret Manager Admin role

2. **"Secret not found"**: Check if Terraform has created the database password secret

### Migration Issues

1. **"no migration"**: Check that migration files exist in `internal/pkg/database/migrations/`

2. **"dirty database"**: Use `--force VERSION` to reset, then re-run migrations

## Environment Files Structure

The `.env.staging` and `.env.prod` files should contain:

```bash
# OAuth Credentials
GOOGLE_CLIENT_ID=xxx
GOOGLE_CLIENT_SECRET=xxx
STRAVA_CLIENT_ID=xxx
STRAVA_CLIENT_SECRET=xxx

# Security Secrets
JWT_SECRET=xxx
ENCRYPTION_SECRET=xxx

# Email Configuration
SMTP_USERNAME=xxx
SMTP_PASSWORD=xxx
FROM_EMAIL=xxx

# Redis URL (optional - will be auto-constructed if using Terraform Memorystore)
REDIS_URL=redis://host:port

# URLs (update based on your deployment)
BASE_URL=https://api.yourdomain.com
FRONTEND_URL=https://yourdomain.com
```

The database URL and Redis URL (when using Memorystore) are automatically constructed from Terraform outputs and don't need to be in the env file.