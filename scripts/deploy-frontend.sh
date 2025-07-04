#!/bin/bash

# deploy-frontend.sh - Build and deploy frontend to GCS bucket
# Usage: ./deploy-frontend.sh [staging|prod]

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
WEB_DIR="$PROJECT_ROOT/web"
TERRAFORM_DIR="$PROJECT_ROOT/terraform"

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
    echo "Usage: $0 [staging|prod]"
    echo ""
    echo "Environments:"
    echo "  staging - Deploy to staging environment"
    echo "  prod    - Deploy to production environment"
    echo ""
    echo "This script will:"
    echo "  1. Build the React application"
    echo "  2. Get the frontend bucket name from Terraform"
    echo "  3. Deploy the built files to the GCS bucket"
    echo ""
    exit 1
}

# Check arguments
if [ $# -lt 1 ]; then
    usage
fi

ENVIRONMENT=$1

# Validate environment
if [ "$ENVIRONMENT" != "staging" ] && [ "$ENVIRONMENT" != "prod" ]; then
    print_error "Invalid environment: $ENVIRONMENT"
    usage
fi

# Check if web directory exists
if [ ! -d "$WEB_DIR" ]; then
    print_error "Web directory not found: $WEB_DIR"
    exit 1
fi

# Build the React application
print_info "Building React application..."
cd "$WEB_DIR"

# Install dependencies if needed
if [ ! -d "node_modules" ]; then
    print_info "Installing dependencies..."
    npm install
fi

# Get backend API URL from Terraform for environment variables
print_info "Getting backend API URL from Terraform..."
cd "$TERRAFORM_DIR"
terraform workspace select "$ENVIRONMENT" > /dev/null 2>&1

# Try to get custom domain URL first, fallback to regular URL
BACKEND_API_URL=$(terraform output -raw backend_api_custom_domain 2>/dev/null) || {
    # Fallback to regular Cloud Run URL
    BACKEND_API_URL=$(terraform output -raw backend_api_url 2>/dev/null) || {
        print_error "Failed to get backend API URL. Have you deployed the backend?"
        exit 1
    }
}
print_info "Backend API URL: $BACKEND_API_URL"

# Get OAuth client IDs from Google Secret Manager
print_info "Fetching OAuth client IDs from Secret Manager..."
GOOGLE_CLIENT_ID=$(gcloud secrets versions access latest --secret="google-client-id" --project="$PROJECT_ID" 2>/dev/null) || {
    print_error "Failed to get Google client ID from Secret Manager"
    exit 1
}
STRAVA_CLIENT_ID=$(gcloud secrets versions access latest --secret="strava-client-id" --project="$PROJECT_ID" 2>/dev/null) || {
    print_error "Failed to get Strava client ID from Secret Manager"
    exit 1
}

# Build the application
cd "$WEB_DIR"
print_info "Running build for static export..."
print_info "Note: Next.js will use 'output: export' mode for static hosting"

# Set environment variables for the build
export NEXT_PUBLIC_API_URL="$BACKEND_API_URL"
export NEXT_PUBLIC_GOOGLE_CLIENT_ID="$GOOGLE_CLIENT_ID"
export NEXT_PUBLIC_STRAVA_CLIENT_ID="$STRAVA_CLIENT_ID"

# Ensure we're building for static export (not standalone)
unset BUILD_STANDALONE

if npm run build; then
    print_success "React application built successfully"
else
    print_error "Failed to build React application"
    exit 1
fi

# Get the output directory - Next.js with output: 'export' creates 'out' directory
BUILD_DIR="out"

# Check if the build directory exists
if [ ! -d "$BUILD_DIR" ]; then
    print_error "Build output directory '$BUILD_DIR' not found."
    print_info "Make sure your next.config.mjs has 'output: export' and the build completed successfully."
    print_info "Next.js static export creates an 'out' directory by default."
    exit 1
fi

print_info "Using build directory: $BUILD_DIR"

# Get bucket name from Terraform
print_info "Getting frontend bucket name from Terraform..."
cd "$TERRAFORM_DIR"

# Select the correct workspace
print_info "Selecting Terraform workspace: $ENVIRONMENT"
terraform workspace select "$ENVIRONMENT" > /dev/null 2>&1 || {
    print_error "Failed to select Terraform workspace. Have you run 'terraform init' and created the workspace?"
    exit 1
}

# Get the bucket name
BUCKET_NAME=$(terraform output -raw frontend_bucket_name 2>/dev/null) || {
    print_error "Failed to get frontend bucket name. Have you deployed the infrastructure?"
    print_info "Run the following commands first:"
    echo "  cd terraform"
    echo "  terraform workspace select $ENVIRONMENT"
    echo "  terraform apply -var-file=\"$ENVIRONMENT.tfvars\""
    exit 1
}

if [ -z "$BUCKET_NAME" ]; then
    print_error "Frontend bucket name is empty. Infrastructure may not be deployed."
    exit 1
fi

print_info "Deploying to bucket: gs://$BUCKET_NAME"

# Deploy to GCS bucket
cd "$WEB_DIR"
print_info "Syncing files to GCS..."

# Use gsutil rsync to sync files
# -r: Recursive
# -d: Delete files in destination that don't exist in source
# -c: Compare checksums instead of mtime
if gsutil -m rsync -r -d -c "$BUILD_DIR/" "gs://$BUCKET_NAME/"; then
    print_success "Files deployed successfully!"
else
    print_error "Failed to deploy files to GCS"
    exit 1
fi

# Set proper cache headers for different file types
print_info "Setting cache headers..."

# HTML files - no cache
gsutil -m setmeta -h "Cache-Control:no-cache, no-store, must-revalidate" "gs://$BUCKET_NAME/**.html" 2>/dev/null || true

# Static assets - long cache
gsutil -m setmeta -h "Cache-Control:public, max-age=31536000, immutable" "gs://$BUCKET_NAME/**.js" 2>/dev/null || true
gsutil -m setmeta -h "Cache-Control:public, max-age=31536000, immutable" "gs://$BUCKET_NAME/**.css" 2>/dev/null || true
gsutil -m setmeta -h "Cache-Control:public, max-age=31536000, immutable" "gs://$BUCKET_NAME/**.png" 2>/dev/null || true
gsutil -m setmeta -h "Cache-Control:public, max-age=31536000, immutable" "gs://$BUCKET_NAME/**.jpg" 2>/dev/null || true
gsutil -m setmeta -h "Cache-Control:public, max-age=31536000, immutable" "gs://$BUCKET_NAME/**.jpeg" 2>/dev/null || true
gsutil -m setmeta -h "Cache-Control:public, max-age=31536000, immutable" "gs://$BUCKET_NAME/**.gif" 2>/dev/null || true
gsutil -m setmeta -h "Cache-Control:public, max-age=31536000, immutable" "gs://$BUCKET_NAME/**.svg" 2>/dev/null || true
gsutil -m setmeta -h "Cache-Control:public, max-age=31536000, immutable" "gs://$BUCKET_NAME/**.woff" 2>/dev/null || true
gsutil -m setmeta -h "Cache-Control:public, max-age=31536000, immutable" "gs://$BUCKET_NAME/**.woff2" 2>/dev/null || true

# Get the frontend URL
cd "$TERRAFORM_DIR"
FRONTEND_URL=$(terraform output -raw frontend_url 2>/dev/null) || {
    print_warning "Could not get frontend URL from Terraform"
}

# Summary
print_success "Frontend deployed successfully!"
echo ""
echo "Deployment details:"
echo "  Environment: $ENVIRONMENT"
echo "  Bucket: gs://$BUCKET_NAME"
if [ -n "$FRONTEND_URL" ]; then
    echo "  URL: $FRONTEND_URL"
fi
echo ""
echo "Notes:"
echo "  - It may take a few minutes for changes to propagate through the CDN"
echo "  - SSL certificate provisioning can take up to 60 minutes for new domains"
echo "  - Check certificate status with: gcloud compute ssl-certificates describe $ENVIRONMENT-frontend-ssl-cert"