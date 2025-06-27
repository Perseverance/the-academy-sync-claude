#!/bin/bash

# migrate-db.sh - Run database migrations for The Academy Sync
# Usage: ./migrate-db.sh [staging|prod] [--proxy]

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ID="the-academy-sync-sdlc-test"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"
MIGRATIONS_PATH="$PROJECT_ROOT/internal/pkg/database/migrations"

# Function to print colored output
print_error() {
    echo -e "${RED}Error: $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

# Function to display usage
usage() {
    echo "Usage: $0 [staging|prod] [options]"
    echo ""
    echo "Environments:"
    echo "  staging - Run migrations on staging database"
    echo "  prod    - Run migrations on production database"
    echo ""
    echo "Options:"
    echo "  --proxy  - Use Cloud SQL Proxy for connection (recommended)"
    echo "  --down N - Rollback N migrations"
    echo "  --force VERSION - Force database to specific version"
    echo "  --status - Show current migration status"
    echo ""
    echo "Examples:"
    echo "  $0 staging                  # Run all pending migrations on staging"
    echo "  $0 prod --proxy            # Run migrations on prod using Cloud SQL Proxy"
    echo "  $0 staging --status        # Check migration status on staging"
    echo "  $0 prod --down 1           # Rollback last migration on prod"
    exit 1
}

# Check if migrate is installed
if ! command -v migrate &> /dev/null; then
    print_error "migrate command not found. Please install it:"
    echo "  brew install golang-migrate"
    exit 1
fi

# Check arguments
if [ $# -lt 1 ]; then
    usage
fi

ENVIRONMENT=$1
shift

# Validate environment
if [ "$ENVIRONMENT" != "staging" ] && [ "$ENVIRONMENT" != "prod" ]; then
    print_error "Invalid environment: $ENVIRONMENT"
    usage
fi

# Parse additional options
USE_PROXY=false
MIGRATION_COMMAND="up"
MIGRATION_ARGS=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --proxy)
            USE_PROXY=true
            shift
            ;;
        --down)
            MIGRATION_COMMAND="down"
            MIGRATION_ARGS="$2"
            shift 2
            ;;
        --force)
            MIGRATION_COMMAND="force"
            MIGRATION_ARGS="$2"
            shift 2
            ;;
        --status)
            MIGRATION_COMMAND="version"
            shift
            ;;
        *)
            print_error "Unknown option: $1"
            usage
            ;;
    esac
done

# Function to get database URL
get_database_url() {
    local ENV=$1
    local USE_PROXY=$2
    
    print_info "Fetching database configuration..."
    
    cd "$PROJECT_ROOT/terraform" || exit 1
    
    # Select the correct workspace
    terraform workspace select "$ENV" >/dev/null 2>&1
    
    if [ "$USE_PROXY" == "true" ]; then
        # Get instance connection name for proxy
        INSTANCE_CONNECTION=$(terraform output -raw db_instance_connection_name 2>/dev/null || echo "")
        
        if [ -z "$INSTANCE_CONNECTION" ]; then
            print_error "Could not fetch instance connection name from Terraform"
            exit 1
        fi
        
        # Get database info
        DB_NAME=$(terraform output -raw db_name 2>/dev/null || echo "")
        DB_USER=$(terraform output -raw db_user 2>/dev/null || echo "")
        
        # Get password from Secret Manager
        DB_PASSWORD=$(gcloud secrets versions access latest --secret="${ENV}-db-password" --project="$PROJECT_ID" 2>/dev/null || echo "")
        
        if [ -z "$DB_PASSWORD" ]; then
            print_error "Could not fetch database password from Secret Manager"
            exit 1
        fi
        
        # URL-encode the password
        ENCODED_PASSWORD=$(python3 -c "import urllib.parse; print(urllib.parse.quote('''$DB_PASSWORD'''))")
        
        # Start Cloud SQL Proxy
        print_info "Starting Cloud SQL Proxy..."
        
        # Check if cloud-sql-proxy is installed
        if ! command -v cloud-sql-proxy &> /dev/null; then
            print_error "cloud-sql-proxy not found. Installing..."
            curl -o /tmp/cloud-sql-proxy https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.11.0/cloud-sql-proxy.darwin.amd64
            chmod +x /tmp/cloud-sql-proxy
            PROXY_CMD="/tmp/cloud-sql-proxy"
        else
            PROXY_CMD="cloud-sql-proxy"
        fi
        
        # Kill any existing proxy on port 5433
        lsof -ti:5433 | xargs kill -9 2>/dev/null || true
        
        # Start proxy in background
        $PROXY_CMD --port=5433 "$INSTANCE_CONNECTION" &
        PROXY_PID=$!
        
        # Wait for proxy to start
        sleep 3
        
        # Export proxy PID for cleanup
        export PROXY_PID
        
        # Construct URL for proxy connection
        echo "postgres://${DB_USER}:${ENCODED_PASSWORD}@localhost:5433/${DB_NAME}?sslmode=disable"
    else
        # Direct connection
        DB_IP=$(terraform output -raw db_instance_ip 2>/dev/null | jq -r '.[0].ip_address' 2>/dev/null || echo "")
        DB_NAME=$(terraform output -raw db_name 2>/dev/null || echo "")
        DB_USER=$(terraform output -raw db_user 2>/dev/null || echo "")
        
        if [ -z "$DB_IP" ]; then
            print_error "Could not fetch database IP from Terraform"
            exit 1
        fi
        
        # Get password from Secret Manager
        DB_PASSWORD=$(gcloud secrets versions access latest --secret="${ENV}-db-password" --project="$PROJECT_ID" 2>/dev/null || echo "")
        
        if [ -z "$DB_PASSWORD" ]; then
            print_error "Could not fetch database password from Secret Manager"
            exit 1
        fi
        
        # URL-encode the password
        ENCODED_PASSWORD=$(python3 -c "import urllib.parse; print(urllib.parse.quote('''$DB_PASSWORD'''))")
        
        # Construct direct connection URL
        echo "postgres://${DB_USER}:${ENCODED_PASSWORD}@${DB_IP}:5432/${DB_NAME}?sslmode=require"
    fi
}

# Cleanup function
cleanup() {
    if [ -n "$PROXY_PID" ]; then
        print_info "Stopping Cloud SQL Proxy..."
        kill $PROXY_PID 2>/dev/null || true
    fi
}

# Set trap for cleanup
trap cleanup EXIT

# Main execution
print_info "Running database migrations for $ENVIRONMENT environment..."

# Get database URL
DATABASE_URL=$(get_database_url "$ENVIRONMENT" "$USE_PROXY")

if [ -z "$DATABASE_URL" ]; then
    print_error "Failed to construct database URL"
    exit 1
fi

# Check if migrations directory exists
if [ ! -d "$MIGRATIONS_PATH" ]; then
    print_error "Migrations directory not found: $MIGRATIONS_PATH"
    exit 1
fi

# Run migration command
print_info "Executing migration command: $MIGRATION_COMMAND $MIGRATION_ARGS"

case "$MIGRATION_COMMAND" in
    up)
        migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" up
        print_success "Migrations completed successfully!"
        ;;
    down)
        if [ -z "$MIGRATION_ARGS" ]; then
            print_error "Please specify number of migrations to rollback"
            exit 1
        fi
        migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" down "$MIGRATION_ARGS"
        print_success "Rolled back $MIGRATION_ARGS migration(s)"
        ;;
    force)
        if [ -z "$MIGRATION_ARGS" ]; then
            print_error "Please specify version to force"
            exit 1
        fi
        print_warning "Forcing database to version $MIGRATION_ARGS"
        migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" force "$MIGRATION_ARGS"
        print_success "Database forced to version $MIGRATION_ARGS"
        ;;
    version)
        echo "Current migration version:"
        migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" version
        ;;
esac

# Show current status after changes
if [ "$MIGRATION_COMMAND" != "version" ]; then
    echo ""
    print_info "Current migration status:"
    migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" version || true
fi