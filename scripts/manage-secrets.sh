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
REGION="europe-central2"
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

print_info() {
    echo -e "${NC}ℹ $1${NC}"
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
    
    # List all secrets (no environment prefix)
    SECRETS=$(gcloud secrets list --project="$PROJECT_ID" --format="value(name)" || true)
    
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
    print_info "Fetching database information from Terraform..." >&2
    
    cd "$PROJECT_ROOT/terraform" || exit 1
    
    # Select the correct workspace
    terraform workspace select "$ENV" >/dev/null 2>&1
    
    # Get database outputs
    DB_CONNECTION_NAME=$(terraform output -raw db_instance_connection_name 2>/dev/null || echo "")
    DB_NAME=$(terraform output -raw db_name 2>/dev/null || echo "")
    DB_USER=$(terraform output -raw db_user 2>/dev/null || echo "")
    
    if [ -z "$DB_CONNECTION_NAME" ] || [ -z "$DB_NAME" ] || [ -z "$DB_USER" ]; then
        print_warning "Could not fetch database info from Terraform. Database URL secret will need to be updated manually." >&2
        return
    fi
    
    # Get the database password from Secret Manager
    DB_PASSWORD=$(gcloud secrets versions access latest --secret="db-password" --project="$PROJECT_ID" 2>/dev/null || echo "")
    
    if [ -z "$DB_PASSWORD" ]; then
        print_warning "Could not fetch database password. Database URL secret will need to be updated manually." >&2
        return
    fi
    
    # URL-encode the password (special characters need encoding)
    ENCODED_PASSWORD=$(python3 -c "import urllib.parse; print(urllib.parse.quote('''$DB_PASSWORD'''))")
    
    # Get the private IP address of the database
    print_info "Fetching database private IP..." >&2
    DB_PRIVATE_IP=$(gcloud sql instances describe "${ENV}-primary-instance" --project="$PROJECT_ID" --format=json | jq -r '.ipAddresses[] | select(.type=="PRIVATE") | .ipAddress' 2>/dev/null || echo "")
    
    if [ -z "$DB_PRIVATE_IP" ]; then
        print_warning "Could not fetch database private IP. Using Cloud SQL socket format instead." >&2
        # Fallback to Cloud SQL socket connection format
        DATABASE_URL="postgres://${DB_USER}:${ENCODED_PASSWORD}@/${DB_NAME}?host=/cloudsql/${DB_CONNECTION_NAME}"
    else
        # Use direct private IP connection (works better with Cloud Run)
        DATABASE_URL="postgres://${DB_USER}:${ENCODED_PASSWORD}@${DB_PRIVATE_IP}:5432/${DB_NAME}?sslmode=disable"
        print_info "Using private IP connection: ${DB_PRIVATE_IP}" >&2
    fi
    
    echo "$DATABASE_URL"
}

# Function to construct Redis URL
construct_redis_url() {
    local ENV=$1
    
    # Get Redis connection info from Terraform
    print_info "Fetching Redis information from Terraform..." >&2
    
    cd "$PROJECT_ROOT/terraform" || exit 1
    
    # Select the correct workspace
    terraform workspace select "$ENV" >/dev/null 2>&1
    
    # Get Redis outputs
    REDIS_HOST=$(terraform output -raw redis_host 2>/dev/null || echo "")
    REDIS_PORT=$(terraform output -raw redis_port 2>/dev/null || echo "")
    
    if [ -z "$REDIS_HOST" ] || [ -z "$REDIS_PORT" ]; then
        print_warning "Could not fetch Redis info from Terraform. Redis URL secret will need to be updated manually." >&2
        return
    fi
    
    # Get the Redis auth string directly from the Redis instance
    print_info "Fetching Redis auth string from Redis instance..." >&2
    REDIS_AUTH=$(gcloud redis instances get-auth-string "${ENV}-redis-instance" --region="$REGION" --project="$PROJECT_ID" --format="value(authString)" 2>/dev/null || echo "")
    
    if [ -z "$REDIS_AUTH" ]; then
        print_warning "Could not fetch Redis auth string from instance. Redis URL secret will need to be updated manually." >&2
        return
    fi
    
    # Also update the redis-auth secret with the actual value
    print_info "Updating Redis auth secret..." >&2
    echo -n "$REDIS_AUTH" | gcloud secrets versions add "redis-auth" --data-file=- --project="$PROJECT_ID" >/dev/null 2>&1
    
    # Construct the Redis URL with TLS support for Google Cloud Memorystore
    # Format: rediss://:[password]@[host]:[port]/[database]
    # Note: rediss:// protocol enables TLS
    REDIS_URL="rediss://:${REDIS_AUTH}@${REDIS_HOST}:${REDIS_PORT}/0"
    
    echo "$REDIS_URL"
}

# Function to construct BASE_URL from Cloud Run backend-api service
construct_base_url() {
    local ENV=$1
    
    # Get backend API URL from Terraform
    print_info "Fetching backend API URL from Terraform..." >&2
    
    cd "$PROJECT_ROOT/terraform" || exit 1
    
    # Select the correct workspace
    terraform workspace select "$ENV" >/dev/null 2>&1
    
    # Try to get custom domain URL first, fallback to regular URL
    BACKEND_URL=$(terraform output -raw backend_api_custom_domain 2>/dev/null || echo "")
    
    if [ -z "$BACKEND_URL" ]; then
        # Fallback to regular Cloud Run URL
        BACKEND_URL=$(terraform output -raw backend_api_url 2>/dev/null || echo "")
    fi
    
    if [ -z "$BACKEND_URL" ]; then
        print_warning "Could not fetch backend API URL from Terraform. BASE_URL secret will need to be updated manually." >&2
        return
    fi
    
    echo "$BACKEND_URL"
}

# Function to construct FRONTEND_URL from Terraform outputs
construct_frontend_url() {
    local ENV=$1
    
    # Get frontend URL from Terraform
    print_info "Fetching frontend URL from Terraform..." >&2
    
    cd "$PROJECT_ROOT/terraform" || exit 1
    
    # Select the correct workspace
    terraform workspace select "$ENV" >/dev/null 2>&1
    
    # Get frontend URL
    FRONTEND_URL=$(terraform output -raw frontend_url 2>/dev/null || echo "")
    
    if [ -z "$FRONTEND_URL" ]; then
        print_warning "Could not fetch frontend URL from Terraform. FRONTEND_URL secret will need to be updated manually." >&2
        return
    fi
    
    echo "$FRONTEND_URL"
}

# Main logic
case "$COMMAND" in
    create|update)
        print_info "Processing secrets for $ENVIRONMENT environment..." >&2
        echo "" >&2
        
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
                    manage_secret "google-client-id" "$value" "$COMMAND"
                    ;;
                GOOGLE_CLIENT_SECRET)
                    manage_secret "google-client-secret" "$value" "$COMMAND"
                    ;;
                STRAVA_CLIENT_ID)
                    manage_secret "strava-client-id" "$value" "$COMMAND"
                    ;;
                STRAVA_CLIENT_SECRET)
                    manage_secret "strava-client-secret" "$value" "$COMMAND"
                    ;;
                JWT_SECRET)
                    # Generate JWT secret if it's a placeholder
                    if [[ "$value" == *"change-this"* ]] || [[ "$value" == "your-"* ]]; then
                        print_warning "Generating secure JWT secret..."
                        value=$(openssl rand -base64 32)
                        print_success "Generated JWT secret"
                    fi
                    manage_secret "jwt-secret" "$value" "$COMMAND"
                    ;;
                ENCRYPTION_SECRET)
                    # Generate encryption secret if it's a placeholder
                    if [[ "$value" == *"change-this"* ]] || [[ "$value" == "your-"* ]]; then
                        print_warning "Generating secure encryption secret..."
                        value=$(openssl rand -base64 48)
                        print_success "Generated encryption secret"
                    fi
                    manage_secret "encryption-secret" "$value" "$COMMAND"
                    ;;
                SMTP_USERNAME)
                    manage_secret "smtp-username" "$value" "$COMMAND"
                    ;;
                SMTP_PASSWORD)
                    manage_secret "smtp-password" "$value" "$COMMAND"
                    ;;
                FROM_EMAIL)
                    manage_secret "from-email" "$value" "$COMMAND"
                    ;;
                REDIS_URL)
                    # Skip if placeholder value, we'll construct it from Terraform
                    if [[ "$value" == "redis://"* ]] && [[ "$value" == *"redis-staging"* || "$value" == *"redis-prod"* ]]; then
                        print_warning "Skipping placeholder Redis URL, will construct from Terraform outputs"
                    else
                        manage_secret "redis-url" "$value" "$COMMAND"
                    fi
                    ;;
                BASE_URL)
                    # Skip if placeholder value, we'll construct it from Terraform outputs
                    if [[ "$value" == *"staging-api"* || "$value" == *"api.yourdomain"* ]]; then
                        print_warning "Skipping placeholder BASE_URL, will construct from Cloud Run outputs"
                    else
                        manage_secret "base-url" "$value" "$COMMAND"
                    fi
                    ;;
                FRONTEND_URL)
                    # Skip if placeholder value, we'll construct it from Terraform outputs
                    if [[ "$value" == *"localhost"* || "$value" == *"yourdomain"* || "$value" == *"staging.theacademysync"* || "$value" == *"theacademysync.run"* ]]; then
                        print_warning "Skipping placeholder FRONTEND_URL, will construct from Terraform outputs"
                    else
                        manage_secret "frontend-url" "$value" "$COMMAND"
                    fi
                    ;;
            esac
        done < "$ENV_FILE"
        
        # Handle database URL specially
        echo "" >&2
        print_info "Constructing database URL..." >&2
        DB_URL=$(construct_database_url "$ENVIRONMENT")
        if [ -n "$DB_URL" ]; then
            manage_secret "database-url" "$DB_URL" "$COMMAND"
        else
            print_warning "Database URL will need to be created manually after Terraform deployment"
        fi
        
        # Handle Redis URL from Terraform
        echo "" >&2
        print_info "Constructing Redis URL..." >&2
        REDIS_URL=$(construct_redis_url "$ENVIRONMENT")
        if [ -n "$REDIS_URL" ]; then
            manage_secret "redis-url" "$REDIS_URL" "$COMMAND"
        else
            print_warning "Redis URL will need to be created manually after Terraform deployment"
        fi
        
        # Handle BASE_URL from Cloud Run
        echo "" >&2
        print_info "Constructing BASE_URL from Cloud Run..." >&2
        BASE_URL=$(construct_base_url "$ENVIRONMENT")
        if [ -n "$BASE_URL" ]; then
            manage_secret "base-url" "$BASE_URL" "$COMMAND"
        else
            print_warning "BASE_URL will need to be created manually after Cloud Run deployment"
        fi
        
        # Handle FRONTEND_URL from Terraform
        echo "" >&2
        print_info "Constructing FRONTEND_URL from Terraform..." >&2
        FRONTEND_URL=$(construct_frontend_url "$ENVIRONMENT")
        if [ -n "$FRONTEND_URL" ]; then
            manage_secret "frontend-url" "$FRONTEND_URL" "$COMMAND"
        else
            print_warning "FRONTEND_URL will need to be created manually after Terraform deployment"
        fi
        
        echo "" >&2
        print_success "Secret management completed for $ENVIRONMENT environment!"
        ;;
        
    view)
        view_secrets
        ;;
        
    *)
        usage
        ;;
esac