# Deploy Scheduler Infrastructure to Staging

This guide walks through deploying the Cloud Scheduler infrastructure for automated processing to the staging environment.

## Prerequisites

1. **GCP Access**: Ensure you have appropriate permissions for the staging project (`the-academy-sync-sdlc-test`)
2. **Terraform**: Have Terraform installed locally
3. **gcloud CLI**: Authenticated with the staging project
4. **Code Changes**: Ensure all scheduler-related code changes are merged to main

## Deployment Steps

### 1. Build and Push Docker Images

First, ensure the latest code with scheduler endpoint is built and pushed:

```bash
cd scripts
./build-and-push-images.sh staging
```

This will build and push images for:
- backend-api (includes the `/tasks/invoke-scheduler` endpoint)
- automation-engine (includes scheduled processing logic)
- notification-service

### 2. Run Database Migrations

If there are any new migrations (there shouldn't be for the scheduler feature):

```bash
cd scripts
./migrate-db.sh staging
```

### 3. Deploy Infrastructure with Terraform

```bash
cd terraform

# Initialize Terraform (if not already done)
terraform init

# Select or create the staging workspace
terraform workspace select staging || terraform workspace new staging

# Review the planned changes
terraform plan -var-file=staging.tfvars

# Apply the changes
terraform apply -var-file=staging.tfvars
```

The Terraform apply will:
- Create the Cloud Scheduler service account
- Grant it permission to invoke the backend-api Cloud Run service
- Create the hourly Cloud Scheduler job
- Configure retry policies and authentication

### 4. Deploy Updated Services

After Terraform creates the infrastructure, deploy the updated services:

```bash
# Deploy backend-api with scheduler endpoint
gcloud run deploy staging-backend-api \
  --image gcr.io/the-academy-sync-sdlc-test/backend-api:latest \
  --region europe-central2 \
  --project the-academy-sync-sdlc-test

# Deploy automation-engine with scheduled processing
gcloud run deploy staging-automation-engine \
  --image gcr.io/the-academy-sync-sdlc-test/automation-engine:latest \
  --region europe-central2 \
  --project the-academy-sync-sdlc-test
```

### 5. Verify Deployment

#### Check Cloud Scheduler Job
```bash
# List scheduler jobs
gcloud scheduler jobs list --location=europe-central2 --project=the-academy-sync-sdlc-test

# Describe the job
gcloud scheduler jobs describe staging-automation-scheduler \
  --location=europe-central2 \
  --project=the-academy-sync-sdlc-test
```

#### Test the Scheduler Endpoint Manually
```bash
# Get the backend API URL
BACKEND_URL=$(gcloud run services describe staging-backend-api \
  --region=europe-central2 \
  --project=the-academy-sync-sdlc-test \
  --format='value(status.url)')

# Test the endpoint (will require authentication)
curl -X POST "$BACKEND_URL/tasks/invoke-scheduler" \
  -H "Authorization: Bearer $(gcloud auth print-identity-token)" \
  -H "Content-Type: application/json"
```

#### Manually Trigger the Scheduler Job
```bash
# Force run the scheduler job
gcloud scheduler jobs run staging-automation-scheduler \
  --location=europe-central2 \
  --project=the-academy-sync-sdlc-test
```

### 6. Monitor Initial Runs

#### Check Cloud Scheduler Logs
```bash
# View scheduler job logs
gcloud logging read "resource.type=cloud_scheduler_job AND resource.labels.job_id=staging-automation-scheduler" \
  --limit=10 \
  --project=the-academy-sync-sdlc-test \
  --format=json
```

#### Check Backend API Logs
```bash
# View backend-api logs for scheduler endpoint
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=staging-backend-api AND textPayload:\"invoke-scheduler\"" \
  --limit=20 \
  --project=the-academy-sync-sdlc-test
```

#### Check Automation Engine Logs
```bash
# View automation-engine logs for scheduled processing
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=staging-automation-engine AND jsonPayload.trigger_type=\"scheduled\"" \
  --limit=20 \
  --project=the-academy-sync-sdlc-test
```

## Post-Deployment Checklist

- [ ] Cloud Scheduler job created and visible in console
- [ ] Service account has correct permissions
- [ ] Backend API `/tasks/invoke-scheduler` endpoint responds
- [ ] Manual trigger of scheduler job succeeds
- [ ] Automation engine processes scheduled jobs
- [ ] No errors in Cloud Scheduler logs
- [ ] Activity logs show scheduled processing (check database)

## Rollback Plan

If issues arise:

1. **Disable the Scheduler Job**:
   ```bash
   gcloud scheduler jobs pause staging-automation-scheduler \
     --location=europe-central2 \
     --project=the-academy-sync-sdlc-test
   ```

2. **Revert Service Deployments** (if needed):
   ```bash
   # Deploy previous version
   gcloud run deploy staging-backend-api \
     --image gcr.io/the-academy-sync-sdlc-test/backend-api:previous-tag \
     --region europe-central2 \
     --project the-academy-sync-sdlc-test
   ```

3. **Remove Scheduler Infrastructure** (if needed):
   ```bash
   cd terraform
   terraform destroy -target=google_cloud_scheduler_job.automation_scheduler -var-file=staging.tfvars
   ```

## Notes

- The scheduler runs every hour at the top of the hour (UTC)
- It will only process users who are in their 3-5 AM local time window
- The placeholder auth middleware will accept the OIDC token from Cloud Scheduler
- Monitor the first few runs closely to ensure proper operation
- Consider setting up alerts for scheduler failures in production