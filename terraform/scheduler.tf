# Cloud Scheduler Configuration for Automated Processing

# Service account for Cloud Scheduler
resource "google_service_account" "scheduler" {
  account_id   = "${var.environment}-scheduler-sa"
  display_name = "Cloud Scheduler Service Account for ${var.environment}"
  description  = "Service account used by Cloud Scheduler to invoke the backend API scheduler endpoint"
  project      = var.project_id
}

# Grant the service account permission to invoke Cloud Run services
resource "google_cloud_run_service_iam_member" "scheduler_invoker" {
  location = google_cloud_run_service.backend_api.location
  project  = google_cloud_run_service.backend_api.project
  service  = google_cloud_run_service.backend_api.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.scheduler.email}"
}

# Cloud Scheduler job for hourly automation processing
resource "google_cloud_scheduler_job" "automation_scheduler" {
  name        = "${var.environment}-automation-scheduler"
  description = "Hourly job to trigger automated processing for users in their 3-4 AM window"
  project     = var.project_id
  region      = var.region

  # Run every hour at the top of the hour
  schedule = "0 * * * *"

  # Use UTC timezone as specified in the requirements
  time_zone = "Etc/UTC"

  # HTTP target configuration
  http_target {
    # TODO: Update this URL after the backend API is deployed with the scheduler endpoint
    # The URL should point to the /tasks/invoke-scheduler endpoint
    uri = "${google_cloud_run_service.backend_api.status[0].url}/tasks/invoke-scheduler"

    http_method = "POST"

    # Headers
    headers = {
      "Content-Type" = "application/json"
    }

    # Empty body for the POST request
    body = base64encode("{}")

    # OIDC token configuration for authentication
    # This will generate a token that the placeholder auth middleware will accept
    # TODO (TECH-011): Replace with proper OIDC validation in production
    oidc_token {
      service_account_email = google_service_account.scheduler.email
      audience              = google_cloud_run_service.backend_api.status[0].url
    }
  }

  # Retry configuration
  retry_config {
    retry_count          = 3
    min_backoff_duration = "5s"
    max_backoff_duration = "60s"
    max_doublings        = 2
  }

  # Ensure the Cloud Scheduler API is enabled before creating the job
  depends_on = [
    google_project_service.cloudscheduler,
    google_cloud_run_service_iam_member.scheduler_invoker
  ]
}

# Output the scheduler job name and service account for reference
output "scheduler_job_name" {
  value       = google_cloud_scheduler_job.automation_scheduler.name
  description = "The name of the Cloud Scheduler job"
}

output "scheduler_service_account" {
  value       = google_service_account.scheduler.email
  description = "The email of the service account used by Cloud Scheduler"
}