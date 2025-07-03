resource "random_password" "db_password" {
  length           = 16
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

resource "google_secret_manager_secret" "db_password_secret" {
  secret_id = "db-password"
  project   = var.project_id
  replication {
    auto {}
  }
  depends_on = [google_project_service.secretmanager]
}

resource "google_secret_manager_secret_version" "db_password_secret_version" {
  secret      = google_secret_manager_secret.db_password_secret.id
  secret_data = random_password.db_password.result
}

resource "google_sql_database_instance" "main" {
  name             = "${var.environment}-primary-instance"
  project          = var.project_id
  region           = var.region
  database_version = "POSTGRES_15"
  settings {
    tier              = var.db_tier
    disk_size         = var.db_disk_size
    availability_type = var.db_availability_type
    backup_configuration {
      enabled                        = var.db_backups_enabled
      point_in_time_recovery_enabled = var.db_point_in_time_recovery_enabled
    }
    ip_configuration {
      ipv4_enabled    = true  # Enable public IP for migrations
      private_network = google_compute_network.main.id
      enable_private_path_for_google_cloud_services = true
      
      # Restrict access to specific IPs
      dynamic "authorized_networks" {
        for_each = var.authorized_networks
        content {
          name  = authorized_networks.value.name
          value = authorized_networks.value.value
        }
      }
    }
    
    # SSL enforcement for PostgreSQL
    database_flags {
      name  = "cloudsql.iam_authentication"
      value = "on"
    }
  }
  deletion_protection = var.db_deletion_protection
  depends_on          = [
    google_project_service.sqladmin,
    google_service_networking_connection.private_vpc_connection
  ]
}

resource "google_sql_database" "default" {
  name     = "${var.environment}-app-db"
  project  = var.project_id
  instance = google_sql_database_instance.main.name
}

resource "google_sql_user" "default" {
  name     = "${var.environment}-app-user"
  project  = var.project_id
  instance = google_sql_database_instance.main.name
  password = random_password.db_password.result
}

output "db_instance_connection_name" {
  description = "The connection name of the database instance."
  value       = google_sql_database_instance.main.connection_name
  sensitive   = true
}

output "db_instance_ip" {
  description = "The IP address of the database instance."
  value       = google_sql_database_instance.main.ip_address
  sensitive   = true
}

output "db_instance_name" {
  description = "The name of the database instance."
  value       = google_sql_database_instance.main.name
}

output "db_name" {
  description = "The name of the database."
  value       = google_sql_database.default.name
}

output "db_user" {
  description = "The name of the database user."
  value       = google_sql_user.default.name
}

output "secret_name" {
  description = "The name of the secret containing the database password."
  value       = google_secret_manager_secret.db_password_secret.secret_id
}
