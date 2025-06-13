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
├── internal/                 # Shared private Go packages
│   └── pkg/
│       ├── database/         # Shared DB Repository
│       └── queue/            # Shared Queue Client
├── web/                      # React frontend application
├── terraform/                # Infrastructure as Code
├── .github/                  # CI/CD workflows
├── docs/                     # Project documentation
│   ├── BRD.md               # Business Requirements Document
│   └── SDD.md               # System Design Document
├── Dockerfile.go             # Go services Dockerfile
└── docker-compose.yml        # Local development setup
```

## Development Setup

### Prerequisites

- Go 1.21+
- Node.js 18+
- Docker & Docker Compose
- PostgreSQL (for local development)
- Redis (for local development)

### Local Development

1. Clone the repository
2. Copy `.env.example` to `.env` and configure local settings
3. Start local infrastructure: `docker-compose up -d`
4. Install dependencies and start services (commands TBD)

### Common Commands

- `go build` - Build the Go applications
- `go run .` or `go run main.go` - Run applications directly
- `go test ./...` - Run all tests
- `go test -v ./...` - Run tests with verbose output
- `go test -cover ./...` - Run tests with coverage
- `go fmt ./...` - Format Go source files
- `go vet ./...` - Run static analysis

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