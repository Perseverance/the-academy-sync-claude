# The Academy Sync

A fully automated system that seamlessly bridges the gap between an athlete's recorded activities on Strava and their coach's prescribed training log in Google Sheets.

## Overview

The Academy Sync eliminates the tedious, error-prone, and time-consuming task of manually transferring training data from Strava to Google Sheets. By automating this process, athletes can focus purely on their training and recovery, knowing their data is being meticulously managed in the background.

### Key Features

- **Automated Data Transfer**: Fetches run data from Strava and logs it to Google Sheets according to coach-prescribed formatting rules
- **Intelligent Processing**: Handles complex workout descriptions, RPE calculations, and data aggregation
- **7-Day Lookback**: Automatically processes missed entries from the past week
- **Smart Scheduling**: Processes data based on user's local timezone (3:00-3:59 AM window)
- **Manual Sync**: On-demand processing via web interface
- **Email Notifications**: Daily summary emails with processing results
- **Multi-User Ready**: Architected to support multiple users with isolated processing

## Architecture

The system follows a microservices architecture deployed on Google Cloud Platform:

### Components

- **Web App**: React SPA for user configuration and monitoring
- **Backend API**: Go service handling authentication and configuration
- **Automation Engine**: Go service for core data processing
- **Notification Service**: Go service for email delivery
- **Database**: PostgreSQL for user data and logs
- **Job Queues**: Redis for asynchronous processing

### Technology Stack

- **Backend**: Go with Chi framework
- **Frontend**: React
- **Database**: PostgreSQL
- **Queues**: Redis
- **Cloud Platform**: Google Cloud Platform
- **Authentication**: Google OAuth 2.0
- **External APIs**: Strava API, Google Sheets API
- **Email**: SendGrid
- **Infrastructure**: Terraform

## Manual Sync Flow

The Academy Sync now includes a comprehensive manual sync feature that allows users to trigger on-demand synchronization of their Strava activities to Google Sheets.

### How Manual Sync Works

1. **User Trigger**: User clicks "Sync Now" button in the web interface
2. **API Request**: Frontend sends POST request to `/api/sync` endpoint
3. **Validation**: Backend validates user authentication and configuration:
   - Checks JWT authentication
   - Validates Strava connection exists
   - Validates Google Spreadsheet is configured
4. **Job Enqueueing**: Valid requests are enqueued to Redis `jobs_queue`
5. **Worker Processing**: Automation engine workers dequeue and process jobs
6. **Data Transfer**: Activities are fetched from Strava and written to Google Sheets
7. **Completion**: Job processing results are logged for monitoring

### Queue-Based Architecture

The manual sync uses a robust Redis-based queue system:

- **Producer**: Backend API enqueues sync jobs
- **Consumer**: Automation engine dequeues jobs with configurable worker pool
- **Queue**: Redis `jobs_queue` ensures FIFO processing and persistence
- **Scaling**: Configurable `MAX_WORKERS` (1-1000, default: 20)

### Worker Processing Pipeline

Each sync job processes multiple days of activities based on the sync type:

**Manual Sync Processing:**
- Today's activities (from midnight to current time)
- Yesterday's activities (full day)
- 7-day lookback period (days 2-8 in the past)

**Scheduled Sync Processing:**
- Yesterday's activities (full day)
- 7-day lookback period (days 2-8 in the past)

The processing follows these steps:

1. **🚀 Starting automation processing**: Initialize job with context and OAuth credentials
2. **📋 Step 1: Retrieving user configuration**: Load user settings and validate automation is enabled
3. **🏃 Step 2: Creating Strava API client**: Initialize Strava client with token management
4. **📊 Step 3: Creating Google Sheets API client**: Initialize Sheets client with token management
5. **🔐 Step 4: Validating Google Sheets access**: Verify spreadsheet permissions
6. **📊 Step 5: Processing data based on job type**: Execute the appropriate sync logic
7. **🎉 Successfully completed**: Log summary with total activities processed

**Activity Count in Logs:**
The `activity_count` shown in logs represents the **total number of activities processed across all days** in the current sync operation, not just a single day. For example:
- 2 activities today + 3 activities yesterday + 2 from lookback = `activity_count: 7`

### Configuration Requirements

#### Redis Configuration

```bash
# Required environment variables
REDIS_URL=redis://redis:6379        # Redis connection string
MAX_WORKERS=20                       # Worker pool size (1-1000)
```

#### Worker Pool Scaling

- **Development**: 5-10 workers typically sufficient
- **Production**: 20-50 workers depending on load
- **API Rate Limits**: Consider Strava (600 requests/15min) and Google Sheets quotas
- **Memory Usage**: Each worker uses ~10-20MB of memory

### API Endpoints

#### Manual Sync Trigger
```http
POST /api/sync
Authorization: Bearer <jwt-token>
Content-Type: application/json

Response (202 Accepted):
{
  "status": "accepted",
  "message": "Sync request has been queued for processing"
}
```

#### Sync Status Check
```http
GET /api/sync/status
Authorization: Bearer <jwt-token>

Response (200 OK):
{
  "eligible": true,
  "reason": ""
}
```

### Error Handling

The system includes comprehensive error handling:

- **Validation Errors**: 400 Bad Request for missing Strava/Sheets configuration
- **Authentication Errors**: 401 Unauthorized for invalid/expired tokens
- **Service Errors**: 503 Service Unavailable for Redis connection issues
- **OAuth Reauth**: Automatic detection and handling of expired OAuth tokens

### Monitoring and Logging

All sync operations include detailed logging:

- **Queue Operations**: Job enqueue/dequeue with timestamps and trace IDs
- **Processing Steps**: Step-by-step progress logging with performance metrics
- **Error Details**: Comprehensive error context for troubleshooting
- **Token Management**: OAuth token validity and refresh status
- **API Interactions**: External API call logging and rate limit tracking

**Monitoring Job Processing:**

```bash
# Watch automation engine logs for job processing
docker-compose logs -f automation-engine | grep -E "(Processing job|Successfully completed|ERROR)"

# Monitor queue operations
docker-compose logs -f backend-api automation-engine | grep -E "(enqueue|dequeue|jobs_queue)"

# Check specific user processing
docker-compose logs automation-engine | grep "user_id\":1"
```

**Key Log Messages to Monitor:**

- `"Successfully enqueued job"` - Job added to queue by backend API
- `"Processing job"` - Worker picked up job from queue
- `"Successfully completed automation processing"` - Job finished with activity count
- `"Failed to retrieve user configuration"` - User config issues
- `"Google Sheets access requires re-authorization"` - OAuth token expired

### Health Checks

The system performs startup health checks:

- **Database Connectivity**: PostgreSQL connection validation with retries
- **Redis Connectivity**: Redis queue connection validation with retries
- **OAuth Configuration**: Google and Strava client credential validation
- **Fail-Fast Behavior**: System exits if critical dependencies are unavailable

## Project Structure

```
/the-academy-sync/
├── cmd/                      # Main Go applications
│   ├── backend-api/
│   ├── automation-engine/
│   └── notification-service/
├── internal/                 # Shared private Go packages (TBD)
│   └── pkg/
│       ├── database/         # Shared DB Repository
│       └── queue/            # Shared Queue Client
├── web/                      # React frontend application (Next.js)
├── terraform/                # Infrastructure as Code (TBD)
├── .github/                  # CI/CD workflows (TBD)
├── docs/                     # Project documentation
│   ├── BRD.md               # Business Requirements Document
│   └── SDD.md               # System Design Document
├── Dockerfile                # Multi-stage Dockerfile for Go services
├── docker-compose.yml        # Local development setup
├── go.mod                    # Go module definition
└── go.sum                    # Go module checksums
```

## Development Setup

### Prerequisites

- Go 1.23+
- Node.js 18+
- Docker & Docker Compose
- PostgreSQL (for local development)
- Redis (for local development)

### Local Development with Docker Compose

**Quick Start:**

1. **Clone the repository:**
   ```bash
   git clone https://github.com/Perseverance/the-academy-sync-claude.git
   cd the-academy-sync-claude
   ```

2. **Configure environment:**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. **Start the entire application stack:**
   ```bash
   docker-compose up --build
   ```

4. **Access the applications:**
   - Web UI: http://localhost:3000
   - Backend API: http://localhost:8080
   - PostgreSQL: localhost:5433
   - Redis: localhost:6380

**Development Commands:**

```bash
# Start all services in the background
docker-compose up -d

# View logs from all services
docker-compose logs -f

# View logs from a specific service
docker-compose logs -f backend-api

# Stop all services
docker-compose down

# Stop and remove volumes (data will be lost)
docker-compose down -v

# Rebuild and restart all services
docker-compose up --build

# Restart a specific service
docker-compose restart backend-api
```

**Live Reloading:**

The Go services are configured with Air for automatic live reloading during development. When you modify Go source files, the affected service will automatically rebuild and restart.

**Note:** The Air configuration excludes test files (`*_test.go`, `test_*.go`, `debug_*.go`) from triggering rebuilds to prevent unnecessary restarts during debugging.

**Database Persistence:**

PostgreSQL data is persisted in a Docker volume. Your data will survive container restarts but will be lost if you run `docker-compose down -v`.

### Troubleshooting Redis Queue Issues

If jobs are not being processed:

1. **Check for competing connections:**
   ```bash
   # Check BRPOP connections
   docker-compose exec redis redis-cli CLIENT LIST | grep "cmd=brpop"
   
   # Check for host processes
   ps aux | grep -E "automation-engine|automatio" | grep -v docker
   ```

2. **Kill any host processes that might be consuming jobs:**
   ```bash
   # If you find automation-engine processes on host
   kill <PID>
   ```

3. **Verify only Docker containers are connected:**
   - There should be exactly 1 BRPOP connection from the automation-engine container
   - No connections should come from the host machine (172.x.0.1)

### Manual Development Setup

Alternatively, you can run services individually for development:

### Building Docker Images

Build container images for each service:

#### Go Services
Use the multi-stage Dockerfile for Go services:

```bash
# Backend API
docker build --build-arg SERVICE_NAME=backend-api -t the-academy-sync-backend-api .

# Automation Engine
docker build --build-arg SERVICE_NAME=automation-engine -t the-academy-sync-automation-engine .

# Notification Service
docker build --build-arg SERVICE_NAME=notification-service -t the-academy-sync-notification-service .
```

#### React Web UI
Build and run the React frontend:

```bash
# Build the web application
cd web
docker build -t academy-sync-web .

# Run the web application
docker run -p 8080:8080 academy-sync-web
```

The web application will be available at `http://localhost:8080`.

## Configuration Management

The Academy Sync uses a hybrid configuration loading strategy that supports both local development and production environments.

### Environment Detection

The system automatically detects the environment using the following priority:

1. `APP_ENV` environment variable
2. `GO_ENV` environment variable (fallback)
3. Default to `local`

### Configuration Loading

- **Local/Development** (`APP_ENV=local`, `development`, or `dev`): Loads from `.env` file and environment variables
- **Production/Staging** (`APP_ENV=production` or `staging`): Loads from Google Secret Manager with environment variable fallback

### Required Environment Variables

#### Core Configuration
- `APP_ENV` - Environment name (`local`, `development`, `production`, etc.)
- `PORT` - Service port (default: 8080)
- `LOG_LEVEL` - Logging level (`DEBUG`, `INFO`, `WARNING`, `ERROR`, `CRITICAL`) (default: INFO)

#### Database Configuration
- `DATABASE_URL` - Complete PostgreSQL connection string (auto-generated if not provided)
- `POSTGRES_DB` - Database name (default: academy_sync)
- `POSTGRES_USER` - Database username (default: postgres)
- `POSTGRES_PASSWORD` - Database password (required in production)
- `POSTGRES_HOST` - Database host (default: localhost)
- `POSTGRES_PORT` - Database port (default: 5433 for local, 5432 for production)

#### Redis Configuration
- `REDIS_URL` - Complete Redis connection string (auto-generated if not provided)
- `REDIS_HOST` - Redis host (default: localhost)
- `REDIS_PORT` - Redis port (default: 6380 for local, 6379 for production)

#### Worker Pool Configuration
- `MAX_WORKERS` - Maximum concurrent workers for sync job processing (default: 20, range: 1-1000)

#### OAuth Configuration
- `GOOGLE_CLIENT_ID` - Google OAuth client ID
- `GOOGLE_CLIENT_SECRET` - Google OAuth client secret
- `STRAVA_CLIENT_ID` - Strava OAuth client ID
- `STRAVA_CLIENT_SECRET` - Strava OAuth client secret

#### Security Configuration
- `JWT_SECRET` - JWT signing secret (required in production)

#### SMTP Configuration (for notifications)
- `SMTP_HOST` - SMTP server host (default: smtp.gmail.com)
- `SMTP_PORT` - SMTP server port (default: 587)
- `SMTP_USERNAME` - SMTP username
- `SMTP_PASSWORD` - SMTP password
- `FROM_EMAIL` - From email address

#### Google Cloud Configuration
- `GCP_PROJECT_ID` - Google Cloud Project ID (for Secret Manager integration)

### Local Development Setup

1. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` with your configuration values

3. The configuration will be automatically loaded when starting any service

### Configuration Validation

The system performs validation on startup:
- Critical fields must be present
- JWT secret is required in production environments
- Port must be a valid number
- Service will fail to start if validation fails

### Logging Configuration

The Academy Sync uses structured JSON logging powered by Go's `log/slog` package. All logs are output to stdout/stderr for cloud-native deployments.

#### Log Levels

The system supports five log levels controlled by the `LOG_LEVEL` environment variable:

- **DEBUG**: Detailed information for diagnosing problems (includes OAuth flows, database queries, etc.)
- **INFO**: General information about system operation (default level)
- **WARNING**: Warning messages for potential issues
- **ERROR**: Error messages for failures that don't stop execution
- **CRITICAL**: Critical errors that may stop system operation

#### Setting Log Levels

**Docker Compose Development:**
```bash
# Set in environment
LOG_LEVEL=DEBUG docker-compose up

# Or add to .env file
echo "LOG_LEVEL=DEBUG" >> .env
docker-compose up
```

**Direct Go Execution:**
```bash
LOG_LEVEL=DEBUG go run ./cmd/backend-api
```

**Production Deployment:**
Set `LOG_LEVEL` as an environment variable in your deployment configuration.

#### JSON Log Format

All logs are output in structured JSON format:

```json
{
  "time": "2025-06-14T11:51:29.460402+03:00",
  "level": "INFO",
  "msg": "Backend API starting", 
  "service": "backend-api",
  "environment": "development",
  "port": "8080",
  "additional_fields": "..."
}
```

This format enables easy parsing by log aggregation systems like ELK stack, Grafana Loki, or cloud logging services.

### Google Secret Manager Integration

The configuration system includes full Google Secret Manager support for production deployments:

- **Production Mode**: When `APP_ENV=production` and `GCP_PROJECT_ID` is set, the system loads secrets from Google Secret Manager
- **Fallback Behavior**: If Secret Manager is unavailable (no credentials, network issues, etc.), the system gracefully falls back to environment variables
- **Authentication**: Uses Application Default Credentials (ADC) - see [GCP Authentication docs](https://cloud.google.com/docs/authentication/external/set-up-adc)
- **Logging**: Provides clear feedback about Secret Manager connection status and number of secrets loaded

**Secret Naming Convention**:
- `database-url` - Complete database connection string
- `database-password` - Database password (for URL construction)
- `redis-url` - Complete Redis connection string  
- `google-client-id` / `google-client-secret` - OAuth credentials
- `strava-client-id` / `strava-client-secret` - OAuth credentials
- `jwt-secret` - JWT signing secret
- `smtp-username` / `smtp-password` - Email credentials
- `from-email` - Email sender address

**Example GCP Setup**:
```bash
# Set up Application Default Credentials
gcloud auth application-default login

# Set project for Secret Manager
export GCP_PROJECT_ID=your-project-id
export APP_ENV=production

# Service will now load secrets from Secret Manager
./backend-api
```

## Database Migrations

The Academy Sync uses `golang-migrate/migrate` for database schema management. All migration files are stored in `internal/pkg/database/migrations/`.

### Migration File Naming

Migration files follow the pattern: `NNNNNN_description.up.sql` and `NNNNNN_description.down.sql` where:
- `NNNNNN` is a 6-digit sequence number (e.g., `000001`)
- `description` is a brief description of the migration
- `.up.sql` contains the forward migration (creating/altering tables)
- `.down.sql` contains the rollback migration (undoing the changes)

### Running Migrations

#### Prerequisites

Install the migrate CLI tool:

```bash
# Install migrate CLI
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

#### Database URL Format

The migration commands require a PostgreSQL database URL:

```bash
# Local development (using docker-compose)
export DATABASE_URL="postgres://postgres:password@localhost:5433/academy_sync?sslmode=disable"

# Or for production
export DATABASE_URL="postgres://username:password@host:port/database?sslmode=require"
```

#### Apply Migrations (Up)

```bash
# Apply all pending migrations
migrate -path internal/pkg/database/migrations -database "$DATABASE_URL" up

# Apply a specific number of migrations
migrate -path internal/pkg/database/migrations -database "$DATABASE_URL" up 1
```

#### Rollback Migrations (Down)

```bash
# Rollback the last migration
migrate -path internal/pkg/database/migrations -database "$DATABASE_URL" down 1

# Rollback all migrations (WARNING: This will drop all tables)
migrate -path internal/pkg/database/migrations -database "$DATABASE_URL" down
```

#### Check Migration Status

```bash
# Show current migration version
migrate -path internal/pkg/database/migrations -database "$DATABASE_URL" version

# Check if database is up to date
migrate -path internal/pkg/database/migrations -database "$DATABASE_URL" up
```

#### Force Migration Version (Recovery)

If migrations get into a bad state:

```bash
# Force set the migration version (use with caution)
migrate -path internal/pkg/database/migrations -database "$DATABASE_URL" force VERSION_NUMBER
```

### Creating New Migrations

To create a new migration:

```bash
# Create new migration files
migrate create -ext sql -dir internal/pkg/database/migrations -seq description_of_change

# This creates:
# internal/pkg/database/migrations/NNNNNN_description_of_change.up.sql
# internal/pkg/database/migrations/NNNNNN_description_of_change.down.sql
```

### Migration Best Practices

1. **Always test both up and down migrations** in a development environment
2. **Keep migrations small and focused** on a single logical change
3. **Never edit existing migration files** after they've been applied in production
4. **Use transactions** when possible to ensure atomic operations
5. **Add appropriate indexes** for performance
6. **Include rollback logic** in every down migration

### Docker Compose Integration

When using Docker Compose for local development, migrations are automatically applied when you start the services:

```bash
# Start all services (migrations will run automatically)
docker-compose up

# Or run in the background
docker-compose up -d
```

The `migrate` service will:
1. Wait for PostgreSQL to be ready
2. Apply all pending migrations
3. Exit successfully
4. Allow dependent services (backend-api, automation-engine, etc.) to start

If you need to run migrations manually:

```bash
# Set the local database URL
export DATABASE_URL="postgres://postgres:password@localhost:5433/academy_sync?sslmode=disable"

# Apply migrations
migrate -path internal/pkg/database/migrations -database "$DATABASE_URL" up
```

### Common Development Commands

#### Go Services
- `go build ./cmd/<service-name>` - Build specific Go application
- `go run ./cmd/<service-name>` - Run application directly
- `go test ./...` - Run all tests
- `go test -v ./...` - Run tests with verbose output
- `go test -cover ./...` - Run tests with coverage
- `go fmt ./...` - Format Go source files
- `go vet ./...` - Run static analysis
- `go test ./internal/pkg/config -v` - Test configuration package specifically

#### React Web UI
```bash
cd web

# Install dependencies
npm install
# or
pnpm install

# Start development server
npm run dev
# or
pnpm run dev

# Build for production
npm run build
# or
pnpm run build

# Start production server
npm run start
# or
pnpm run start
```

The development server runs on `http://localhost:3000` by default.

## Documentation

- [Business Requirements Document](docs/BRD.md) - Detailed project requirements and scope
- [System Design Document](docs/SDD.md) - Architecture, design decisions, and technical specifications
- [CLAUDE.md](CLAUDE.md) - AI assistant development guidance

## Deployment

The system is designed for deployment on Google Cloud Platform using:

- **Cloud Run** for Go services
- **Cloud Storage + CDN** for React frontend
- **Cloud SQL** for PostgreSQL
- **Memorystore** for Redis with TLS
- **Cloud Scheduler** for automated triggers
- **Secret Manager** for credential storage

All infrastructure is managed via Terraform in the `terraform/` directory.

### Quick Start Deployment

```bash
# 1. Enable APIs (one-time)
gcloud services enable compute.googleapis.com sqladmin.googleapis.com secretmanager.googleapis.com run.googleapis.com vpcaccess.googleapis.com redis.googleapis.com servicenetworking.googleapis.com --project=<project-id>

# 2. Deploy infrastructure
cd terraform && terraform init && terraform workspace select staging
terraform apply -var-file=staging.tfvars

# 3. Configure secrets
cd ../scripts && cp ../.env.staging.example ../.env.staging
# Edit ../.env.staging with your values
./manage-secrets.sh update staging

# 4. Build and deploy
./build-and-push-images.sh staging
./migrate-db.sh staging

# 5. Redeploy Cloud Run services
cd ../terraform && terraform apply -var-file=staging.tfvars -target=google_cloud_run_service.backend_api -target=google_cloud_run_service.automation_engine -target=google_cloud_run_service.notification_service
```

### Prerequisites

Before deploying, ensure you have:

1. **Google Cloud SDK** installed and configured
2. **Terraform** v1.5+ installed
3. **Docker** installed and authenticated to GCR (`gcloud auth configure-docker`)
4. **Go** 1.23+ installed (for building services)
5. **Authenticated to GCP**: `gcloud auth login` and `gcloud auth application-default login`

### Initial Deployment

Follow these steps for a fresh deployment to a new environment:

#### 1. Enable Google Cloud APIs

**IMPORTANT**: Enable APIs first to avoid "API not enabled" errors during Terraform apply:

```bash
# Enable all required APIs
gcloud services enable \
  compute.googleapis.com \
  sqladmin.googleapis.com \
  secretmanager.googleapis.com \
  run.googleapis.com \
  vpcaccess.googleapis.com \
  redis.googleapis.com \
  servicenetworking.googleapis.com \
  --project=the-academy-sync-sdlc-test
```

#### 2. Infrastructure Setup

```bash
cd terraform

# Initialize Terraform
terraform init

# Create workspace for your environment
terraform workspace new staging  # or "prod" for production

# Select the workspace
terraform workspace select staging

# Plan infrastructure changes
terraform plan -var-file=staging.tfvars -out=staging.tfplan

# Apply infrastructure
terraform apply staging.tfplan
```

**Note**: The first apply will show Cloud Run deployment failures - this is expected because Docker images don't exist yet.

#### 3. Configure Secrets

Terraform creates secrets with placeholder values. You need to update them with actual values:

```bash
# First, prepare your environment file
cp .env.staging.example .env.staging

# Edit with your actual values:
# - OAuth credentials (Google & Strava)
# - SMTP credentials
# - Frontend URL
vim .env.staging

# Update secrets in Google Secret Manager
cd scripts
./manage-secrets.sh update staging  # Use 'update', not 'create'
```

The script will:
- Read values from `.env.staging`
- Generate secure JWT_SECRET and ENCRYPTION_SECRET if needed
- Construct DATABASE_URL from Terraform outputs
- Construct REDIS_URL with TLS support
- Update all secrets in Google Secret Manager

#### 4. Build and Push Docker Images

```bash
# Build and push all service images
./build-and-push-images.sh staging
```

This builds and pushes:
- `gcr.io/<project-id>/backend-api:staging`
- `gcr.io/<project-id>/automation-engine:staging`
- `gcr.io/<project-id>/notification-service:staging`

#### 5. Database Initialization

Run migrations to set up the database schema:

```bash
# Run database migrations
./migrate-db.sh staging
```

#### 6. Deploy Cloud Run Services

Now that images exist, deploy the Cloud Run services:

```bash
cd ../terraform
terraform apply -var-file=staging.tfvars \
  -target=google_cloud_run_service.backend_api \
  -target=google_cloud_run_service.automation_engine \
  -target=google_cloud_run_service.notification_service
```

#### 7. Verify Deployment

```bash
# Get service URLs
terraform output backend_api_url
terraform output automation_engine_url
terraform output notification_service_url

# Test health endpoints
curl $(terraform output -raw backend_api_url)/health
curl $(terraform output -raw automation_engine_url)/health
curl $(terraform output -raw notification_service_url)/health

# View logs if needed
gcloud run services logs read staging-backend-api --region=europe-central2 --limit=50
```

### Quick Deployment (One-Liner)

For experienced users, enable APIs and deploy infrastructure in one command:

```bash
gcloud services enable compute.googleapis.com sqladmin.googleapis.com secretmanager.googleapis.com run.googleapis.com vpcaccess.googleapis.com redis.googleapis.com servicenetworking.googleapis.com --project=<project-id> && terraform apply -var-file=staging.tfvars
```

### Update Deployment

For updating an existing deployment with new code changes:

#### 1. Code Updates

```bash
# Pull latest changes
git pull origin main
```

#### 2. Infrastructure Updates (if needed)

Only run if infrastructure changes are required:

```bash
cd terraform
terraform workspace select staging
terraform plan -var-file=staging.tfvars
terraform apply -var-file=staging.tfvars
```

#### 3. Database Migrations (if needed)

Only run if there are new migrations:

```bash
# Check current migration version
./scripts/migrate-db.sh staging --status

# Apply new migrations
./scripts/migrate-db.sh staging --verbose
```

#### 4. Deploy Updated Services

```bash
# Build and deploy all services
./scripts/build-and-push-images.sh staging
```

### Common Issues and Solutions

#### API Not Enabled Errors

If you see "API has not been used in project" errors:
```bash
# Enable the specific API
gcloud services enable <api-name>.googleapis.com --project=<project-id>
# Wait 2-3 minutes for propagation
```

#### VPC Peering Deletion Issues

If destroying infrastructure fails with "Failed to delete connection":
```bash
# List and manually delete VPC peerings
gcloud compute networks peerings list --network=<vpc-name> --project=<project-id>
gcloud compute networks peerings delete <peering-name> --network=<vpc-name> --project=<project-id>
```

#### Secret Already Exists

Use `update` instead of `create` when running manage-secrets.sh:
```bash
./manage-secrets.sh update staging  # NOT 'create'
```

### Project Configuration for Production

When deploying to production with a different GCP project:

1. Update `terraform/prod.tfvars`:
   ```hcl
   project_id = "your-production-project-id"
   ```

2. Update `scripts/manage-secrets.sh` line 15:
   ```bash
   PROJECT_ID="your-production-project-id"
   ```

3. Switch Terraform workspace:
   ```bash
   terraform workspace select prod
   ```

### Deployment Sequence Summary

The correct order for deployment operations:

1. **Terraform** - Create/update infrastructure
2. **Secrets** - Configure application secrets
3. **Migrations** - Update database schema
4. **Images** - Build and deploy application code

### Environment-Specific Configurations

#### Staging Environment

- Uses smaller Cloud SQL instance (db-f1-micro)
- Single Cloud Run instance per service
- Lower memory allocations
- Basic monitoring

#### Production Environment

- Uses larger Cloud SQL instance (db-n1-standard-1)
- Multiple Cloud Run instances with autoscaling
- Higher memory allocations
- Full monitoring and alerting

### Monitoring Deployments

Monitor deployment status:

```bash
# Check Cloud Run service status
gcloud run services list --region=us-central1

# View service logs
gcloud run services logs read backend-api --region=us-central1 --limit=50

# Check database connectivity
gcloud sql instances describe academy-sync-db-staging

# Monitor job processing
gcloud logging read "resource.type=cloud_run_revision AND jsonPayload.job_type=manual_sync" --limit=20
```

### Rollback Procedures

If deployment issues occur:

#### Service Rollback

```bash
# List available revisions
gcloud run revisions list --service=backend-api --region=us-central1

# Rollback to previous revision
gcloud run services update-traffic backend-api \
  --to-revisions=backend-api-00001-abc=100 \
  --region=us-central1
```

#### Database Rollback

```bash
# Rollback last migration
./scripts/migrate-db.sh staging --down 1

# Force to specific version (use with caution)
./scripts/migrate-db.sh staging --force 3
```

### Automation Scripts

The deployment process is automated through several scripts in the `scripts/` directory:

- **`build-and-push-images.sh`** - Builds and deploys container images
- **`manage-secrets.sh`** - Manages Google Secret Manager secrets
- **`migrate-db.sh`** - Runs database migrations with various options
- **`run-migrations.sh`** - Used by Docker for containerized migrations

Each script includes:
- Help documentation (`--help` flag)
- Dry-run mode for testing
- Verbose output options
- Error handling and validation

### CI/CD Integration

For automated deployments, integrate these scripts into your CI/CD pipeline:

```yaml
# Example GitHub Actions workflow
steps:
  - name: Deploy to Staging
    run: |
      ./scripts/build-and-push-images.sh staging
      ./scripts/migrate-db.sh staging --verbose
```

## Future Improvements

### Job Status Tracking with Long Polling

Currently, the frontend polls the `/me` endpoint after triggering manual sync to detect new activity logs. A more robust solution would be:

1. **Job ID Return**: Modify the manual sync endpoint to return a job ID
2. **Status Endpoint**: Add `GET /api/sync/job/{jobId}/status` with long polling support
3. **Redis Job Tracking**: Store job status (pending, processing, completed, failed) in Redis
4. **Worker Updates**: Update job status throughout processing lifecycle
5. **Frontend Integration**: Poll status endpoint until job completes, then refresh data

This would provide:
- Real-time job progress tracking
- Reduced unnecessary API calls
- Better error handling and retry logic
- Ability to show processing progress to users

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

The MIT License is a permissive open-source license that allows you to freely use, modify, and distribute this software, provided that the original copyright notice and license are included in all copies or substantial portions of the software.