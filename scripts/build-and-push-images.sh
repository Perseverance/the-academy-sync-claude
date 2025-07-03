#!/bin/bash

# build-and-push-images.sh - Build and push container images for Cloud Run services
# Usage: ./build-and-push-images.sh [staging|prod] [service-name|all]

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
    echo "Usage: $0 [staging|prod] [service-name|all]"
    echo ""
    echo "Environments:"
    echo "  staging - Build with staging tag"
    echo "  prod    - Build with prod tag"
    echo ""
    echo "Services:"
    echo "  backend-api          - Build only backend-api"
    echo "  automation-engine    - Build only automation-engine"
    echo "  notification-service - Build only notification-service"
    echo "  all                  - Build all services (default)"
    echo ""
    echo "Examples:"
    echo "  $0 staging all              # Build all services for staging"
    echo "  $0 prod backend-api         # Build only backend-api for prod"
    exit 1
}

# Check arguments
if [ $# -lt 1 ]; then
    usage
fi

ENVIRONMENT=$1
SERVICE=${2:-all}

# Validate environment
if [ "$ENVIRONMENT" != "staging" ] && [ "$ENVIRONMENT" != "prod" ]; then
    print_error "Invalid environment: $ENVIRONMENT"
    usage
fi

# Validate service
case "$SERVICE" in
    backend-api|automation-engine|notification-service|all)
        ;;
    *)
        print_error "Invalid service: $SERVICE"
        usage
        ;;
esac

# Configure Docker for GCR
print_info "Configuring Docker for Google Container Registry..."
gcloud auth configure-docker --quiet

# Function to build and push a service
build_and_push_service() {
    local SERVICE_NAME=$1
    local TAG="gcr.io/${PROJECT_ID}/${SERVICE_NAME}:${ENVIRONMENT}"
    local LATEST_TAG="gcr.io/${PROJECT_ID}/${SERVICE_NAME}:latest"
    
    print_info "Building ${SERVICE_NAME} for ${ENVIRONMENT}..."
    
    cd "$PROJECT_ROOT"
    
    # Build the image
    if docker build --build-arg SERVICE_NAME="${SERVICE_NAME}" -t "${TAG}" -t "${LATEST_TAG}" .; then
        print_success "Built ${SERVICE_NAME} successfully"
    else
        print_error "Failed to build ${SERVICE_NAME}"
        return 1
    fi
    
    # Push the image with environment tag
    print_info "Pushing ${TAG}..."
    if docker push "${TAG}"; then
        print_success "Pushed ${TAG}"
    else
        print_error "Failed to push ${TAG}"
        return 1
    fi
    
    # Also push with latest tag for convenience
    if [ "$ENVIRONMENT" = "prod" ]; then
        print_info "Pushing ${LATEST_TAG}..."
        if docker push "${LATEST_TAG}"; then
            print_success "Pushed ${LATEST_TAG}"
        else
            print_warning "Failed to push ${LATEST_TAG} (non-critical)"
        fi
    fi
    
    return 0
}

# Main execution
print_info "Building and pushing container images for $ENVIRONMENT environment..."

# Build services based on selection
if [ "$SERVICE" = "all" ]; then
    SERVICES=("backend-api" "automation-engine" "notification-service")
else
    SERVICES=("$SERVICE")
fi

FAILED_SERVICES=()

for svc in "${SERVICES[@]}"; do
    if build_and_push_service "$svc"; then
        print_success "Successfully built and pushed $svc"
    else
        FAILED_SERVICES+=("$svc")
    fi
    echo ""
done

# Summary
if [ ${#FAILED_SERVICES[@]} -eq 0 ]; then
    print_success "All services built and pushed successfully!"
    echo ""
    echo "Next steps:"
    echo "1. Update the image URLs in terraform/${ENVIRONMENT}.tfvars:"
    for svc in "${SERVICES[@]}"; do
        echo "   ${svc//-/_}_image_url = \"gcr.io/${PROJECT_ID}/${svc}:${ENVIRONMENT}\""
    done
    echo ""
    echo "2. Apply Terraform changes:"
    echo "   cd terraform"
    echo "   terraform workspace select ${ENVIRONMENT}"
    echo "   terraform apply -var-file=\"${ENVIRONMENT}.tfvars\""
else
    print_error "Failed to build/push the following services:"
    for svc in "${FAILED_SERVICES[@]}"; do
        echo "  - $svc"
    done
    exit 1
fi