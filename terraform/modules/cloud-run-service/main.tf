# Create a dedicated service account for this Cloud Run service
resource "google_service_account" "service_account" {
  # Shorten service names to fit within 30 character limit
  # staging-backend-api-sa = 22 chars (OK)
  # staging-automation-engine-sa = 28 chars (OK)
  # staging-notification-service-sa = 31 chars (TOO LONG)
  # Use shorter names for longer service names
  account_id = var.service_name == "notification-service" ? "${var.environment}-notif-svc-sa" : "${var.environment}-${var.service_name}-sa"
  display_name = "Service Account for ${var.environment} ${var.service_name}"
  project      = var.project_id
}

# Grant the service account permission to access only its required secrets
resource "google_secret_manager_secret_iam_member" "secret_access" {
  for_each = toset(var.required_secrets)

  project   = var.project_id
  secret_id = each.value
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.service_account.email}"
}

# Cloud Run v2 service
resource "google_cloud_run_v2_service" "service" {
  name     = "${var.environment}-${var.service_name}"
  location = var.region
  project  = var.project_id

  template {
    service_account = google_service_account.service_account.email
    
    # Add annotations including Cloud SQL instance
    annotations = merge(
      var.cloud_sql_connection_name != "" ? {
        "run.googleapis.com/cloudsql-instances" = var.cloud_sql_connection_name
      } : {},
      {
        # Use gen1 for CPU < 1, gen2 for CPU >= 1
        "run.googleapis.com/execution-environment" = tonumber(var.cpu_limit) < 1 ? "gen1" : "gen2"
      }
    )
    
    # Configure VPC connector if provided
    dynamic "vpc_access" {
      for_each = var.vpc_connector_id != "" ? [1] : []
      content {
        connector = var.vpc_connector_id
        egress    = "PRIVATE_RANGES_ONLY"
      }
    }

    # Scaling configuration
    scaling {
      min_instance_count = var.min_instances
      max_instance_count = var.max_instances
    }

    # Container configuration
    containers {
      image = var.image_url

      # Resource limits
      resources {
        limits = {
          cpu    = var.cpu_limit
          memory = var.memory_limit
        }
        cpu_idle = true
      }

      # Plain environment variables
      dynamic "env" {
        for_each = var.env_vars
        content {
          name  = env.key
          value = env.value
        }
      }

      # Secret environment variables
      dynamic "env" {
        for_each = var.secrets
        content {
          name = env.key
          value_source {
            secret_key_ref {
              secret  = env.value
              version = "latest"
            }
          }
        }
      }
    }

    # Request handling
    max_instance_request_concurrency = var.concurrency
    timeout                          = "${var.timeout_seconds}s"
  }

  # Traffic configuration - send 100% to latest revision
  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }

  depends_on = [
    google_secret_manager_secret_iam_member.secret_access
  ]
}

# Allow unauthenticated access if specified
resource "google_cloud_run_service_iam_member" "allow_unauthenticated" {
  count = var.allow_unauthenticated ? 1 : 0

  project  = var.project_id
  location = var.region
  service  = google_cloud_run_v2_service.service.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}