# Terraform Configuration for The Academy Sync

This directory contains the Terraform configuration for deploying The Academy Sync infrastructure.

## Remote State Backend (Google Cloud Storage)

This project is configured to use a Google Cloud Storage (GCS) bucket as a remote backend for storing the Terraform state file (`terraform.tfstate`). This allows for secure state management, consistency, and team collaboration.

### Prerequisites: Manual GCS Bucket Creation

Before you can initialize Terraform and use the remote backend, you **must manually create a GCS bucket** that will store the state file.

### Initial GCP Setup & Authentication

Before creating the GCS bucket or running any Terraform commands that interact with Google Cloud, ensure you have:

1.  **Authenticated with GCP:**
    *   For interactive use, run:
        ```sh
        gcloud auth login
        ```
        And follow the prompts to log in with your Google account.
    *   For non-interactive environments (like CI/CD pipelines), ensure the `GOOGLE_APPLICATION_CREDENTIALS` environment variable is set to the path of your service account key JSON file.

2.  **Enabled the Cloud Storage API:**
    The Google Cloud Storage API must be enabled for your project. You can enable it by running:
    ```sh
    gcloud services enable storage.googleapis.com --project=your-gcp-project-id
    ```
    Replace `your-gcp-project-id` with your actual GCP Project ID.

Once these steps are complete, you can proceed to create the GCS bucket.

**Recommended GCS Bucket Configuration:**

*   **Unique Name:** Choose a globally unique name for your bucket (e.g., `your-unique-project-name-tfstate`).
*   **Location:** Choose a location for your bucket (e.g., `europe-central2`).
*   **Uniform Bucket-Level Access:** Enable this for consistent permission management.
*   **Public Access Prevention:** Ensure "Enforce public access prevention" is ON.
*   **Object Versioning:** Enable object versioning to protect against accidental state deletion or corruption.

**Example `gcloud` command to create such a bucket:**

```sh
gcloud storage buckets create gs://the-academy-sync-claude-tfstate \
    --project=your-gcp-project-id \
    --location=europe-central2 \
    --uniform-bucket-level-access \
    --public-access-prevention
gcloud storage buckets update gs://the-academy-sync-claude-tfstate --versioning
```

Replace `your-gcp-project-id` with your actual project ID. The bucket name `the-academy-sync-claude-tfstate` is used as an example.

### Updating `backend.tf`

Once the bucket is created, you need to update the `terraform/backend.tf` file:

```terraform
terraform {
  backend "gcs" {
    bucket = "the-academy-sync-claude-tfstate"
    prefix = "tf-state"
  }
}
```

The `bucket` attribute should be set to the name of the GCS bucket you created (e.g., `the-academy-sync-claude-tfstate`). The `prefix` is configured to support Terraform workspaces.

### Initializing Terraform

After creating the GCS bucket and updating `backend.tf`, navigate to the `terraform` directory in your terminal and run the following command to initialize Terraform:

```sh
terraform init
```

This command will download the necessary provider plugins and configure the backend to use your GCS bucket. You should see a message like "Successfully configured the backend 'gcs'".

After successful initialization, any `terraform apply` commands will store the state in the configured GCS bucket.

## Managing Environments with Workspaces

This project uses Terraform Workspaces to manage multiple deployment environments (e.g., staging, production) from a single codebase. Each workspace maintains its own state file, allowing for independent and safe infrastructure management.

The backend is configured to store state files in GCS. Terraform automatically creates workspace-specific paths within the bucket (e.g., `gs://the-academy-sync-claude-tfstate/tf-state/env:/staging/`, `gs://the-academy-sync-claude-tfstate/tf-state/env:/prod/`).

### Creating Workspaces

If they don't already exist, you can create workspaces for different environments. For example, to create `staging` and `prod` workspaces:

```sh
terraform workspace new staging
terraform workspace new prod
```

The `default` workspace is present initially. It's recommended to create specific workspaces for your environments rather than using `default` for any particular environment.

### Selecting a Workspace

Before running any Terraform commands for a specific environment, you need to select its workspace:

```sh
terraform workspace select staging
```
Or for production:
```sh
terraform workspace select prod
```
You can list available workspaces with `terraform workspace list`.

### Applying Environment-Specific Configurations

To deploy or make changes to an environment, first select the appropriate workspace. Then, use the `-var-file` flag with `terraform apply` (or `plan`) to specify the environment's configuration file:

**For Staging:**
1. Select the staging workspace:
   ```sh
   terraform workspace select staging
   ```
2. Apply the configuration using `staging.tfvars`:
   ```sh
   terraform plan -var-file="staging.tfvars"
   terraform apply -var-file="staging.tfvars"
   ```

**For Production:**
1. Select the production workspace:
   ```sh
   terraform workspace select prod
   ```
2. Apply the configuration using `prod.tfvars`:
   ```sh
   terraform plan -var-file="prod.tfvars"
   terraform apply -var-file="prod.tfvars"
   ```

Remember to ensure the `bucket` in `backend.tf` is set to your GCS bucket name (e.g., `the-academy-sync-claude-tfstate`) and update the project IDs in `staging.tfvars` and `prod.tfvars` with your actual values.

## Container Images for Cloud Run

Before deploying Cloud Run services, you must build and push the container images to Google Container Registry (GCR):

### Prerequisites for Container Images

1. **Use the build script (recommended):**
   ```sh
   # Build and push all services for staging
   ./scripts/build-and-push-images.sh staging all

   # Or build specific services
   ./scripts/build-and-push-images.sh prod backend-api
   ```

2. **Or manually build and push:**
   ```sh
   # Configure Docker for GCR
   gcloud auth configure-docker

   # Backend API
   docker build --build-arg SERVICE_NAME=backend-api -t gcr.io/the-academy-sync-sdlc-test/backend-api:staging .
   docker push gcr.io/the-academy-sync-sdlc-test/backend-api:staging

   # Automation Engine
   docker build --build-arg SERVICE_NAME=automation-engine -t gcr.io/the-academy-sync-sdlc-test/automation-engine:staging .
   docker push gcr.io/the-academy-sync-sdlc-test/automation-engine:staging

   # Notification Service
   docker build --build-arg SERVICE_NAME=notification-service -t gcr.io/the-academy-sync-sdlc-test/notification-service:staging .
   docker push gcr.io/the-academy-sync-sdlc-test/notification-service:staging
   ```

3. **Update the image URLs in your tfvars files:**
   
   Edit `staging.tfvars` or `prod.tfvars` and replace the placeholder image URLs with your actual image tags:
   ```hcl
   backend_api_image_url          = "gcr.io/the-academy-sync-sdlc-test/backend-api:staging"
   automation_engine_image_url    = "gcr.io/the-academy-sync-sdlc-test/automation-engine:staging"
   notification_service_image_url = "gcr.io/the-academy-sync-sdlc-test/notification-service:staging"
   ```

## Known Issues and Solutions

1. **PORT Environment Variable**: Cloud Run automatically sets the PORT environment variable. Do not set it in your service configuration.

2. **CPU and Concurrency**: For services with CPU < 1 (like notification-service with 0.5 CPU), concurrency is automatically set to 1 as required by Cloud Run.

3. **Service Account Name Length**: Service account IDs must be between 6-30 characters. The notification-service uses a shortened name `notif-svc` to fit within this limit.

4. **Database Access**: The database is configured with private IP only for enhanced security. To access the database for migrations or administration:

   **Option 1: Cloud SQL Proxy (Recommended)**
   ```sh
   # Install Cloud SQL Proxy if not already installed
   curl -o cloud_sql_proxy https://dl.google.com/cloudsql/cloud_sql_proxy.darwin.amd64
   chmod +x cloud_sql_proxy
   
   # Connect to the database
   ./cloud_sql_proxy -instances=PROJECT_ID:REGION:INSTANCE_NAME=tcp:5432
   
   # In another terminal, connect using psql or run migrations
   psql -h localhost -p 5432 -U DB_USER -d DB_NAME
   ```

   **Option 2: IAP Tunneling**
   ```sh
   # Create a tunnel through a Compute Engine instance
   gcloud compute ssh INSTANCE_NAME --tunnel-through-iap --zone=ZONE -- -L 5432:DB_PRIVATE_IP:5432
   
   # Connect to database via the tunnel
   psql -h localhost -p 5432 -U DB_USER -d DB_NAME
   ```

   **Note**: The database no longer has a public IP address and requires SSL connections for security. The authorized_networks configuration is no longer applicable.

## Complete Deployment Guide

This guide covers the full deployment process including infrastructure, backend services, and frontend application.

### Prerequisites

1. **GCP Setup:**
   - Create GCS bucket for Terraform state (see above)
   - Enable required APIs
   - Set up authentication

2. **Local Setup:**
   - Install `terraform`, `gcloud`, `gsutil`
   - Install Node.js and npm (for frontend)
   - Clone the repository

### Step-by-Step Deployment

#### 1. Initialize Terraform

```sh
cd terraform
terraform init
terraform workspace new staging  # or 'prod'
terraform workspace select staging
```

#### 2. Deploy Infrastructure

```sh
# Review the plan
terraform plan -var-file="staging.tfvars"

# Apply the infrastructure
terraform apply -var-file="staging.tfvars"
```

#### 3. Build and Push Container Images

```sh
# Build and push all services
./scripts/build-and-push-images.sh staging all

# Or build specific services
./scripts/build-and-push-images.sh staging backend-api
```

#### 4. Update Terraform with Image URLs

Edit `staging.tfvars` to include the actual image URLs:
```hcl
backend_api_image_url          = "gcr.io/the-academy-sync-sdlc-test/backend-api:staging"
automation_engine_image_url    = "gcr.io/the-academy-sync-sdlc-test/automation-engine:staging"
notification_service_image_url = "gcr.io/the-academy-sync-sdlc-test/notification-service:staging"
```

#### 5. Redeploy Cloud Run Services

```sh
terraform apply -var-file="staging.tfvars"
```

#### 6. Configure Secrets

The manage-secrets script will automatically fetch URLs from Terraform:
```sh
# Create/update all secrets including FRONTEND_URL
./scripts/manage-secrets.sh update staging

# View all secrets
./scripts/manage-secrets.sh view staging
```

#### 7. Run Database Migrations

```sh
./scripts/migrate-db.sh staging
```

#### 8. Configure DNS (if managing DNS zone)

```sh
# Get nameservers
terraform output frontend_dns_nameservers

# Update your domain registrar with the nameservers
# For staging.theacademysync.run, create NS records pointing to these nameservers

# Verify DNS propagation
dig +short NS staging.theacademysync.run
```

#### 9. Deploy Frontend

```sh
# Deploy frontend to GCS bucket
./scripts/deploy-frontend.sh staging

# The script will:
# - Build the React application
# - Get the bucket name from Terraform
# - Deploy files to GCS with proper cache headers
```

#### 10. Verify Deployment

```sh
# Check backend API
curl https://$(terraform output -raw backend_api_url)/health

# Check frontend (after DNS propagates)
curl https://staging.theacademysync.run

# Check SSL certificate status
gcloud compute ssl-certificates describe staging-frontend-ssl-cert --format="get(managed.status)"
```

### Post-Deployment Tasks

1. **Monitor SSL Certificate:**
   - Certificates can take up to 60 minutes to provision
   - Status should show "ACTIVE" when ready

2. **Test Application:**
   - Verify OAuth login works
   - Check all API endpoints
   - Test frontend functionality

3. **Set up Monitoring:**
   - Configure Cloud Monitoring alerts
   - Set up uptime checks
   - Review Cloud Run metrics

### Updating the Application

#### Backend Services Update:
```sh
# Build and push new images
./scripts/build-and-push-images.sh staging backend-api

# Update tfvars with new image URL
# Then apply changes
terraform apply -var-file="staging.tfvars"
```

#### Frontend Update:
```sh
# Simply redeploy
./scripts/deploy-frontend.sh staging
```

### Troubleshooting

#### Frontend Issues:
- **404 errors**: Check if index.html exists in bucket
- **SSL errors**: Verify DNS is properly configured
- **Slow updates**: CDN cache may need time to expire

#### Backend Issues:
- **Connection refused**: Check VPC connector configuration
- **Database errors**: Verify private IP connectivity
- **Secret errors**: Run `manage-secrets.sh view` to check

## Frontend Infrastructure Details

The frontend infrastructure includes:
- Google Cloud Storage bucket for static content
- Global HTTPS Load Balancer with CDN
- Managed SSL certificates
- DNS configuration (when enabled)

### Frontend Architecture

- **Multi-Project Setup**: Staging and production use separate GCP projects
- **Domain Configuration**:
  - Staging: `staging.theacademysync.run`
  - Production: `theacademysync.run` (in separate project)
- **CDN**: Automatic caching with configurable TTLs
- **SSL**: Google-managed certificates with automatic renewal

### Frontend Outputs

After deployment, these outputs are available:
- `frontend_bucket_name`: GCS bucket for static files
- `frontend_load_balancer_ip`: Load balancer IP address
- `frontend_url`: Full HTTPS URL
- `frontend_dns_nameservers`: DNS nameservers (if DNS zone is managed)
