# Secret Manager Resources

# Database URL secret
resource "google_secret_manager_secret" "database_url" {
  secret_id = "database-url"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# Redis URL secret
resource "google_secret_manager_secret" "redis_url" {
  secret_id = "redis-url"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# JWT Secret
resource "google_secret_manager_secret" "jwt_secret" {
  secret_id = "jwt-secret"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# Encryption Secret
resource "google_secret_manager_secret" "encryption_secret" {
  secret_id = "encryption-secret"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# Google OAuth Client ID
resource "google_secret_manager_secret" "google_client_id" {
  secret_id = "google-client-id"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# Google OAuth Client Secret
resource "google_secret_manager_secret" "google_client_secret" {
  secret_id = "google-client-secret"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# Strava OAuth Client ID
resource "google_secret_manager_secret" "strava_client_id" {
  secret_id = "strava-client-id"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# Strava OAuth Client Secret
resource "google_secret_manager_secret" "strava_client_secret" {
  secret_id = "strava-client-secret"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# Base URL
resource "google_secret_manager_secret" "base_url" {
  secret_id = "base-url"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# Frontend URL
resource "google_secret_manager_secret" "frontend_url" {
  secret_id = "frontend-url"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# SendGrid API Key
resource "google_secret_manager_secret" "sendgrid_api_key" {
  secret_id = "sendgrid-api-key"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# From Email
resource "google_secret_manager_secret" "from_email" {
  secret_id = "from-email"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# Redis Auth String
resource "google_secret_manager_secret" "redis_auth" {
  secret_id = "redis-auth"
  project   = var.project_id

  replication {
    auto {}
  }

  depends_on = [google_project_service.secretmanager]
}

# Placeholder secret versions (these will be updated by manage-secrets.sh)
resource "google_secret_manager_secret_version" "database_url" {
  secret      = google_secret_manager_secret.database_url.id
  secret_data = "placeholder"

  lifecycle {
    ignore_changes = [secret_data]
  }
}

resource "google_secret_manager_secret_version" "redis_url" {
  secret      = google_secret_manager_secret.redis_url.id
  secret_data = "placeholder"

  lifecycle {
    ignore_changes = [secret_data]
  }
}

# Generate a random JWT secret
resource "random_password" "jwt_secret" {
  length           = 32
  special          = true
  override_special = "!@#$%^&*()_+-=[]{}|;:,.<>?"
}

resource "google_secret_manager_secret_version" "jwt_secret" {
  secret      = google_secret_manager_secret.jwt_secret.id
  secret_data = base64encode(random_password.jwt_secret.result)

  lifecycle {
    ignore_changes = [secret_data]
  }
}

# Generate a random encryption secret
resource "random_password" "encryption_secret" {
  length           = 48
  special          = true
  override_special = "!@#$%^&*()_+-=[]{}|;:,.<>?"
}

resource "google_secret_manager_secret_version" "encryption_secret" {
  secret      = google_secret_manager_secret.encryption_secret.id
  secret_data = base64encode(random_password.encryption_secret.result)

  lifecycle {
    ignore_changes = [secret_data]
  }
}

resource "google_secret_manager_secret_version" "google_client_id" {
  secret      = google_secret_manager_secret.google_client_id.id
  secret_data = "placeholder"

  lifecycle {
    ignore_changes = [secret_data]
  }
}

resource "google_secret_manager_secret_version" "google_client_secret" {
  secret      = google_secret_manager_secret.google_client_secret.id
  secret_data = "placeholder"

  lifecycle {
    ignore_changes = [secret_data]
  }
}

resource "google_secret_manager_secret_version" "strava_client_id" {
  secret      = google_secret_manager_secret.strava_client_id.id
  secret_data = "placeholder"

  lifecycle {
    ignore_changes = [secret_data]
  }
}

resource "google_secret_manager_secret_version" "strava_client_secret" {
  secret      = google_secret_manager_secret.strava_client_secret.id
  secret_data = "placeholder"

  lifecycle {
    ignore_changes = [secret_data]
  }
}

resource "google_secret_manager_secret_version" "base_url" {
  secret      = google_secret_manager_secret.base_url.id
  secret_data = "placeholder"

  lifecycle {
    ignore_changes = [secret_data]
  }
}

resource "google_secret_manager_secret_version" "frontend_url" {
  secret      = google_secret_manager_secret.frontend_url.id
  secret_data = "placeholder"

  lifecycle {
    ignore_changes = [secret_data]
  }
}

resource "google_secret_manager_secret_version" "sendgrid_api_key" {
  secret      = google_secret_manager_secret.sendgrid_api_key.id
  secret_data = var.sendgrid_api_key

  lifecycle {
    ignore_changes = [secret_data]
  }
}

resource "google_secret_manager_secret_version" "from_email" {
  secret      = google_secret_manager_secret.from_email.id
  secret_data = "placeholder"

  lifecycle {
    ignore_changes = [secret_data]
  }
}

# Redis auth is automatically generated by Redis instance
resource "google_secret_manager_secret_version" "redis_auth" {
  secret      = google_secret_manager_secret.redis_auth.id
  secret_data = google_redis_instance.main.auth_string

  lifecycle {
    ignore_changes = [secret_data]
  }
}