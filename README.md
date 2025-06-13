# The Academy Sync

A fully automated system that seamlessly bridges the gap between an athlete's recorded activities on Strava and their coach's prescribed training log in Google Sheets.

## Overview

The Academy Sync eliminates the tedious, error-prone, and time-consuming task of manually transferring training data from Strava to Google Sheets. By automating this process, athletes can focus purely on their training and recovery, knowing their data is being meticulously managed in the background.

### Key Features

- **Automated Data Transfer**: Fetches run data from Strava and logs it to Google Sheets according to coach-prescribed formatting rules
- **Intelligent Processing**: Handles complex workout descriptions, RPE calculations, and data aggregation
- **7-Day Lookback**: Automatically processes missed entries from the past week
- **Smart Scheduling**: Processes data based on user's local timezone (3-5 AM window)
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

- Go 1.22+
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

**Database Persistence:**

PostgreSQL data is persisted in a Docker volume. Your data will survive container restarts but will be lost if you run `docker-compose down -v`.

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

When using Docker Compose for local development, the database is automatically created. You can run migrations by:

1. **Start the database:**
   ```bash
   docker-compose up -d postgres
   ```

2. **Wait for database to be ready, then run migrations:**
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
- **Memorystore** for Redis
- **Cloud Scheduler** for automated triggers
- **Secret Manager** for credential storage

All infrastructure is managed via Terraform in the `terraform/` directory.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

The MIT License is a permissive open-source license that allows you to freely use, modify, and distribute this software, provided that the original copyright notice and license are included in all copies or substantial portions of the software.