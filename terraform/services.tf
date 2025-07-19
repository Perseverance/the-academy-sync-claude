# Cloud Run Services Configuration

# Backend API Service
resource "google_cloud_run_service" "backend_api" {
  name     = "${var.environment}-backend-api"
  location = var.region
  project  = var.project_id

  template {
    spec {
      containers {
        image = "gcr.io/${var.project_id}/backend-api:${var.environment}"

        resources {
          limits = {
            memory = "512Mi"
            cpu    = "1"
          }
        }

        env {
          name  = "APP_ENV"
          value = var.environment
        }

        env {
          name  = "GCP_PROJECT_ID"
          value = var.project_id
        }

        # Environment variables from secrets
        env {
          name = "DATABASE_URL"
          value_from {
            secret_key_ref {
              name = "database-url"
              key  = "latest"
            }
          }
        }

        env {
          name = "REDIS_URL"
          value_from {
            secret_key_ref {
              name = "redis-url"
              key  = "latest"
            }
          }
        }

        env {
          name = "JWT_SECRET"
          value_from {
            secret_key_ref {
              name = "jwt-secret"
              key  = "latest"
            }
          }
        }

        env {
          name = "GOOGLE_CLIENT_ID"
          value_from {
            secret_key_ref {
              name = "google-client-id"
              key  = "latest"
            }
          }
        }

        env {
          name = "GOOGLE_CLIENT_SECRET"
          value_from {
            secret_key_ref {
              name = "google-client-secret"
              key  = "latest"
            }
          }
        }

        env {
          name = "STRAVA_CLIENT_ID"
          value_from {
            secret_key_ref {
              name = "strava-client-id"
              key  = "latest"
            }
          }
        }

        env {
          name = "STRAVA_CLIENT_SECRET"
          value_from {
            secret_key_ref {
              name = "strava-client-secret"
              key  = "latest"
            }
          }
        }

        env {
          name = "BASE_URL"
          value_from {
            secret_key_ref {
              name = "base-url"
              key  = "latest"
            }
          }
        }

        env {
          name = "FRONTEND_URL"
          value_from {
            secret_key_ref {
              name = "frontend-url"
              key  = "latest"
            }
          }
        }

        env {
          name = "ENCRYPTION_SECRET"
          value_from {
            secret_key_ref {
              name = "encryption-secret"
              key  = "latest"
            }
          }
        }

        env {
          name  = "SMTP_HOST"
          value = "smtp.sendgrid.net"
        }

        env {
          name  = "SMTP_PORT"
          value = "587"
        }
      }

      service_account_name = google_service_account.backend_api_sa.email
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/maxScale"        = "100"
        "run.googleapis.com/cloudsql-instances"   = google_sql_database_instance.main.connection_name
        "run.googleapis.com/vpc-access-connector" = google_vpc_access_connector.main.id
        "run.googleapis.com/vpc-access-egress"    = "private-ranges-only"
        "run.googleapis.com/deletion-protection"  = "true"
      }
    }
  }

  traffic {
    percent         = 100
    latest_revision = true
  }

  depends_on = [
    google_secret_manager_secret_version.database_url,
    google_secret_manager_secret_version.redis_url,
    google_secret_manager_secret_version.jwt_secret,
    google_secret_manager_secret_version.google_client_id,
    google_secret_manager_secret_version.google_client_secret,
    google_secret_manager_secret_version.strava_client_id,
    google_secret_manager_secret_version.strava_client_secret,
    google_secret_manager_secret_version.base_url,
    google_secret_manager_secret_version.frontend_url,
    google_project_service.run,
    google_vpc_access_connector.main
  ]
}

# Automation Engine Service
resource "google_cloud_run_service" "automation_engine" {
  name     = "${var.environment}-automation-engine"
  location = var.region
  project  = var.project_id

  template {
    spec {
      containers {
        image = "gcr.io/${var.project_id}/automation-engine:${var.environment}"

        resources {
          limits = {
            memory = "1Gi"
            cpu    = "2"
          }
        }

        env {
          name  = "APP_ENV"
          value = var.environment
        }

        env {
          name  = "GCP_PROJECT_ID"
          value = var.project_id
        }

        env {
          name  = "MAX_WORKERS"
          value = "20"
        }

        # Environment variables from secrets
        env {
          name = "DATABASE_URL"
          value_from {
            secret_key_ref {
              name = "database-url"
              key  = "latest"
            }
          }
        }

        env {
          name = "REDIS_URL"
          value_from {
            secret_key_ref {
              name = "redis-url"
              key  = "latest"
            }
          }
        }

        env {
          name = "ENCRYPTION_SECRET"
          value_from {
            secret_key_ref {
              name = "encryption-secret"
              key  = "latest"
            }
          }
        }

        env {
          name = "GOOGLE_CLIENT_ID"
          value_from {
            secret_key_ref {
              name = "google-client-id"
              key  = "latest"
            }
          }
        }

        env {
          name = "GOOGLE_CLIENT_SECRET"
          value_from {
            secret_key_ref {
              name = "google-client-secret"
              key  = "latest"
            }
          }
        }

        env {
          name = "STRAVA_CLIENT_ID"
          value_from {
            secret_key_ref {
              name = "strava-client-id"
              key  = "latest"
            }
          }
        }

        env {
          name = "STRAVA_CLIENT_SECRET"
          value_from {
            secret_key_ref {
              name = "strava-client-secret"
              key  = "latest"
            }
          }
        }

        env {
          name = "JWT_SECRET"
          value_from {
            secret_key_ref {
              name = "jwt-secret"
              key  = "latest"
            }
          }
        }

        env {
          name = "BASE_URL"
          value_from {
            secret_key_ref {
              name = "base-url"
              key  = "latest"
            }
          }
        }
      }

      service_account_name = google_service_account.automation_engine_sa.email
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/minScale"        = "1"
        "autoscaling.knative.dev/maxScale"        = "10"
        "run.googleapis.com/cloudsql-instances"   = google_sql_database_instance.main.connection_name
        "run.googleapis.com/vpc-access-connector" = google_vpc_access_connector.main.id
        "run.googleapis.com/vpc-access-egress"    = "private-ranges-only"
        "run.googleapis.com/deletion-protection"  = "true"
      }
    }
  }

  traffic {
    percent         = 100
    latest_revision = true
  }

  depends_on = [
    google_secret_manager_secret_version.database_url,
    google_secret_manager_secret_version.redis_url,
    google_secret_manager_secret_version.encryption_secret,
    google_secret_manager_secret_version.google_client_id,
    google_secret_manager_secret_version.google_client_secret,
    google_secret_manager_secret_version.strava_client_id,
    google_secret_manager_secret_version.strava_client_secret,
    google_project_service.run,
    google_vpc_access_connector.main
  ]
}

# Notification Service
resource "google_cloud_run_service" "notification_service" {
  name     = "${var.environment}-notification-service"
  location = var.region
  project  = var.project_id

  template {
    spec {
      containers {
        image = "gcr.io/${var.project_id}/notification-service:${var.environment}"

        resources {
          limits = {
            memory = "512Mi"
            cpu    = "1"
          }
        }

        env {
          name  = "APP_ENV"
          value = var.environment
        }

        env {
          name  = "GCP_PROJECT_ID"
          value = var.project_id
        }

        # Environment variables from secrets
        env {
          name = "DATABASE_URL"
          value_from {
            secret_key_ref {
              name = "database-url"
              key  = "latest"
            }
          }
        }

        env {
          name = "REDIS_URL"
          value_from {
            secret_key_ref {
              name = "redis-url"
              key  = "latest"
            }
          }
        }

        env {
          name = "ENCRYPTION_SECRET"
          value_from {
            secret_key_ref {
              name = "encryption-secret"
              key  = "latest"
            }
          }
        }

        env {
          name = "SENDGRID_API_KEY"
          value_from {
            secret_key_ref {
              name = "sendgrid-api-key"
              key  = "latest"
            }
          }
        }

        env {
          name = "FROM_EMAIL"
          value_from {
            secret_key_ref {
              name = "from-email"
              key  = "latest"
            }
          }
        }

        env {
          name = "JWT_SECRET"
          value_from {
            secret_key_ref {
              name = "jwt-secret"
              key  = "latest"
            }
          }
        }

        env {
          name = "BASE_URL"
          value_from {
            secret_key_ref {
              name = "base-url"
              key  = "latest"
            }
          }
        }
      }

      service_account_name = google_service_account.notification_service_sa.email
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/minScale"        = "1"
        "autoscaling.knative.dev/maxScale"        = "5"
        "run.googleapis.com/cloudsql-instances"   = google_sql_database_instance.main.connection_name
        "run.googleapis.com/vpc-access-connector" = google_vpc_access_connector.main.id
        "run.googleapis.com/vpc-access-egress"    = "private-ranges-only"
        "run.googleapis.com/deletion-protection"  = "true"
      }
    }
  }

  traffic {
    percent         = 100
    latest_revision = true
  }

  depends_on = [
    google_secret_manager_secret_version.database_url,
    google_secret_manager_secret_version.redis_url,
    google_secret_manager_secret_version.encryption_secret,
    google_secret_manager_secret_version.sendgrid_api_key,
    google_secret_manager_secret_version.from_email,
    google_project_service.run,
    google_vpc_access_connector.main,
    google_secret_manager_secret_iam_member.notification_service_secrets
  ]
}

# Service Accounts
resource "google_service_account" "backend_api_sa" {
  account_id   = "${var.environment}-backend-api-sa"
  display_name = "Backend API Service Account"
  project      = var.project_id
}

resource "google_service_account" "automation_engine_sa" {
  account_id   = "${var.environment}-automation-engine-sa"
  display_name = "Automation Engine Service Account"
  project      = var.project_id
}

resource "google_service_account" "notification_service_sa" {
  account_id   = "${var.environment}-notification-svc-sa"
  display_name = "Notification Service Account"
  project      = var.project_id
}

# IAM bindings for service accounts
resource "google_project_iam_member" "backend_api_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.backend_api_sa.email}"
}

resource "google_project_iam_member" "automation_engine_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.automation_engine_sa.email}"
}

resource "google_project_iam_member" "notification_service_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.notification_service_sa.email}"
}

# Secret Manager access for service accounts
resource "google_secret_manager_secret_iam_member" "backend_api_secrets" {
  for_each = toset([
    "database-url",
    "redis-url",
    "jwt-secret",
    "encryption-secret",
    "google-client-id",
    "google-client-secret",
    "strava-client-id",
    "strava-client-secret",
    "base-url",
    "frontend-url",
  ])

  secret_id = each.value
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.backend_api_sa.email}"
  project   = var.project_id
}

resource "google_secret_manager_secret_iam_member" "automation_engine_secrets" {
  for_each = toset([
    "database-url",
    "redis-url",
    "encryption-secret",
    "jwt-secret",
    "base-url",
    "google-client-id",
    "google-client-secret",
    "strava-client-id",
    "strava-client-secret",
  ])

  secret_id = each.value
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.automation_engine_sa.email}"
  project   = var.project_id
}

resource "google_secret_manager_secret_iam_member" "notification_service_secrets" {
  for_each = toset([
    "database-url",
    "redis-url",
    "encryption-secret",
    "jwt-secret",
    "base-url",
    "sendgrid-api-key",
    "from-email",
  ])

  secret_id = each.value
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.notification_service_sa.email}"
  project   = var.project_id
}

# Allow unauthenticated access to backend API
resource "google_cloud_run_service_iam_member" "backend_api_invoker" {
  service  = google_cloud_run_service.backend_api.name
  location = google_cloud_run_service.backend_api.location
  project  = var.project_id
  role     = "roles/run.invoker"
  member   = "allUsers"
}