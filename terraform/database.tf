resource "random_password" "db_password" {
  length           = 16
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

resource "google_secret_manager_secret" "db_password_secret" {
  secret_id = "db-password"
  project   = var.project_id
  replication {
    automatic = true
  }
}

resource "google_secret_manager_secret_version" "db_password_secret_version" {
  secret      = google_secret_manager_secret.db_password_secret.id
  secret_data = random_password.db_password.result
}

resource "google_sql_database_instance" "default" {
  name             = "primary-instance"
  project          = var.project_id
  region           = var.region
  database_version = "POSTGRES_15"
  settings {
    tier    = var.db_tier
    disk_size = var.db_disk_size
    availability_type = var.db_availability_type
    backup_configuration {
      enabled            = var.db_backups_enabled
      point_in_time_recovery_enabled = var.db_point_in_time_recovery_enabled
    }
    ip_configuration {
      authorized_networks {
        value           = "0.0.0.0/0" # FIXME: This should be restricted
        name            = "Allow all"
      }
    }
  }
  deletion_protection = var.db_deletion_protection
}

resource "google_sql_database" "default" {
  name     = "app-db"
  project  = var.project_id
  instance = google_sql_database_instance.default.name
}

resource "google_sql_user" "default" {
  name     = "app-user"
  project  = var.project_id
  instance = google_sql_database_instance.default.name
  password = random_password.db_password.result
}

output "db_instance_connection_name" {
  description = "The connection name of the database instance."
  value       = google_sql_database_instance.default.connection_name
  sensitive   = true
}
