#!/bin/bash

# manage-secrets.sh - Manage Google Secret Manager secrets for The Academy Sync
# Usage: ./manage-secrets.sh [create|view|update] [staging|prod]

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ID="the-academy-sync-sdlc-test"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

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

# Function to display usage
usage() {
    echo "Usage: $0 [create|view|update] [staging|prod]"
    echo ""
    echo "Commands:"
    echo "  create  - Create new secrets from .env file"
    echo "  view    - View existing secrets"
    echo "  update  - Update existing secrets from .env file"
    echo ""
    echo "Environments:"
    echo "  staging - Use .env.staging file"
    echo "  prod    - Use .env.prod file"
    exit 1
}

# Check arguments
if [ $# -ne 2 ]; then
    usage
fi

COMMAND=$1
ENVIRONMENT=$2

# Validate environment
if [ "$ENVIRONMENT" != "staging" ] && [ "$ENVIRONMENT" != "prod" ]; then
    print_error "Invalid environment: $ENVIRONMENT"
    usage
fi

# Check if env file exists
ENV_FILE="$PROJECT_ROOT/.env.$ENVIRONMENT"
if [ "$COMMAND" != "view" ] && [ ! -f "$ENV_FILE" ]; then
    print_error "Environment file not found: $ENV_FILE"
    echo "Please create $ENV_FILE based on .env.$ENVIRONMENT.example"
    exit 1
fi

# Function to create or update a secret
manage_secret() {
    local SECRET_NAME="$1"
    local SECRET_VALUE="$2"
    local ACTION="$3"
    
    if [ "$ACTION" == "create" ]; then
        # Check if secret already exists
        if gcloud secrets describe "$SECRET_NAME" --project="$PROJECT_ID" >/dev/null 2>&1; then
            print_warning "Secret $SECRET_NAME already exists. Use 'update' command to modify it."
            return
        fi
        
        echo -n "$SECRET_VALUE" | gcloud secrets create "$SECRET_NAME" \
            --data-file=- \
            --project="$PROJECT_ID" \
            --replication-policy="automatic" >/dev/null 2>&1
        
        print_success "Created secret: $SECRET_NAME"
    elif [ "$ACTION" == "update" ]; then
        # Check if secret exists
        if ! gcloud secrets describe "$SECRET_NAME" --project="$PROJECT_ID" >/dev/null 2>&1; then
            print_warning "Secret $SECRET_NAME doesn't exist. Creating it now."
            echo -n "$SECRET_VALUE" | gcloud secrets create "$SECRET_NAME" \
                --data-file=- \
                --project="$PROJECT_ID" \
                --replication-policy="automatic" >/dev/null 2>&1
            print_success "Created secret: $SECRET_NAME"
        else
            echo -n "$SECRET_VALUE" | gcloud secrets versions add "$SECRET_NAME" \
                --data-file=- \
                --project="$PROJECT_ID" >/dev/null 2>&1
            print_success "Updated secret: $SECRET_NAME"
        fi
    fi
}

# Function to view secrets
view_secrets() {
    echo "Secrets for $ENVIRONMENT environment:"
    echo ""
    
    # List all secrets with the environment prefix
    SECRETS=$(gcloud secrets list --project="$PROJECT_ID" --format="value(name)" | grep "^${ENVIRONMENT}-" || true)
    
    if [ -z "$SECRETS" ]; then
        print_warning "No secrets found for $ENVIRONMENT environment"
        return
    fi
    
    for SECRET in $SECRETS; do
        echo "• $SECRET"
        if [ "$VERBOSE" == "true" ]; then
            VALUE=$(gcloud secrets versions access latest --secret="$SECRET" --project="$PROJECT_ID" 2>/dev/null || echo "[ERROR: Cannot access]")
            echo "  Value: $VALUE"
            echo ""
        fi
    done
    
    echo ""
    echo "To view secret values, run with VERBOSE=true:"
    echo "VERBOSE=true $0 view $ENVIRONMENT"
}

# Function to construct database URL
construct_database_url() {
    local ENV=$1
    
    # Get database connection info from Terraform
    echo "Fetching database information from Terraform..."
    
    cd "$PROJECT_ROOT/terraform" || exit 1
    
    # Select the correct workspace
    terraform workspace select "$ENV" >/dev/null 2>&1
    
    # Get database outputs
    DB_IP=$(terraform output -raw db_instance_ip 2>/dev/null | jq -r '.[0].ip_address' 2>/dev/null || echo "")
    DB_NAME=$(terraform output -raw db_name 2>/dev/null || echo "")
    DB_USER=$(terraform output -raw db_user 2>/dev/null || echo "")
    
    if [ -z "$DB_IP" ] || [ -z "$DB_NAME" ] || [ -z "$DB_USER" ]; then
        print_warning "Could not fetch database info from Terraform. Database URL secret will need to be updated manually."
        return
    fi
    
    # Get the database password from Secret Manager
    DB_PASSWORD=$(gcloud secrets versions access latest --secret="${ENV}-db-password" --project="$PROJECT_ID" 2>/dev/null || echo "")
    
    if [ -z "$DB_PASSWORD" ]; then
        print_warning "Could not fetch database password. Database URL secret will need to be updated manually."
        return
    fi
    
    # URL-encode the password
    ENCODED_PASSWORD=$(python3 -c "import urllib.parse; print(urllib.parse.quote('''$DB_PASSWORD'''))")
    
    # Construct the database URL
    DATABASE_URL="postgres://${DB_USER}:${ENCODED_PASSWORD}@${DB_IP}:5432/${DB_NAME}?sslmode=require"
    
    echo "$DATABASE_URL"
}

# Main logic
case "$COMMAND" in
    create|update)
        echo "Processing secrets for $ENVIRONMENT environment..."
        echo ""
        
        # Read the env file and process each line
        while IFS='=' read -r key value; do
            # Skip empty lines and comments
            [[ -z "$key" || "$key" =~ ^#.*$ ]] && continue
            
            # Remove leading/trailing whitespace
            key=$(echo "$key" | xargs)
            value=$(echo "$value" | xargs)
            
            # Remove quotes if present
            value="${value%\"}"
            value="${value#\"}"
            value="${value%\'}"
            value="${value#\'}"
            
            # Map env variables to secret names
            case "$key" in
                GOOGLE_CLIENT_ID)
                    manage_secret "${ENVIRONMENT}-google-client-id" "$value" "$COMMAND"
                    ;;
                GOOGLE_CLIENT_SECRET)
                    manage_secret "${ENVIRONMENT}-google-client-secret" "$value" "$COMMAND"
                    ;;
                STRAVA_CLIENT_ID)
                    manage_secret "${ENVIRONMENT}-strava-client-id" "$value" "$COMMAND"
                    ;;
                STRAVA_CLIENT_SECRET)
                    manage_secret "${ENVIRONMENT}-strava-client-secret" "$value" "$COMMAND"
                    ;;
                JWT_SECRET)
                    # Generate JWT secret if it's a placeholder
                    if [[ "$value" == *"change-this"* ]] || [[ "$value" == "your-"* ]]; then
                        print_warning "Generating secure JWT secret..."
                        value=$(openssl rand -base64 32)
                        print_success "Generated JWT secret"
                    fi
                    manage_secret "${ENVIRONMENT}-jwt-secret" "$value" "$COMMAND"
                    ;;
                ENCRYPTION_SECRET)
                    # Generate encryption secret if it's a placeholder
                    if [[ "$value" == *"change-this"* ]] || [[ "$value" == "your-"* ]]; then
                        print_warning "Generating secure encryption secret..."
                        value=$(openssl rand -base64 48)
                        print_success "Generated encryption secret"
                    fi
                    manage_secret "${ENVIRONMENT}-encryption-secret" "$value" "$COMMAND"
                    ;;
                SMTP_USERNAME)
                    manage_secret "${ENVIRONMENT}-smtp-username" "$value" "$COMMAND"
                    ;;
                SMTP_PASSWORD)
                    manage_secret "${ENVIRONMENT}-smtp-password" "$value" "$COMMAND"
                    ;;
                FROM_EMAIL)
                    manage_secret "${ENVIRONMENT}-from-email" "$value" "$COMMAND"
                    ;;
                REDIS_URL)
                    # TODO: When Redis/Memorystore is added to Terraform, derive this from Terraform outputs
                    # similar to how DATABASE_URL is constructed
                    manage_secret "${ENVIRONMENT}-redis-url" "$value" "$COMMAND"
                    ;;
                # TODO: When application infrastructure is added to Terraform:
                # BASE_URL)
                #     # Derive from Cloud Run service URL or Load Balancer IP
                #     manage_secret "${ENVIRONMENT}-base-url" "$value" "$COMMAND"
                #     ;;
                # FRONTEND_URL)
                #     # Derive from Storage Bucket URL or CDN endpoint
                #     manage_secret "${ENVIRONMENT}-frontend-url" "$value" "$COMMAND"
                #     ;;
            esac
        done < "$ENV_FILE"
        
        # Handle database URL specially
        echo ""
        echo "Constructing database URL..."
        DB_URL=$(construct_database_url "$ENVIRONMENT")
        if [ -n "$DB_URL" ]; then
            manage_secret "${ENVIRONMENT}-database-url" "$DB_URL" "$COMMAND"
        else
            print_warning "Database URL will need to be created manually after Terraform deployment"
        fi
        
        echo ""
        print_success "Secret management completed for $ENVIRONMENT environment!"
        ;;
        
    view)
        view_secrets
        ;;
        
    *)
        usage
        ;;
esac