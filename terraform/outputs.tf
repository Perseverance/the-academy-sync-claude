# Terraform Outputs

# Database outputs
output "database_instance_name" {
  description = "The name of the Cloud SQL instance"
  value       = google_sql_database_instance.main.name
}

output "database_connection_name" {
  description = "The connection name of the Cloud SQL instance"
  value       = google_sql_database_instance.main.connection_name
}

output "database_private_ip" {
  description = "The private IP address of the Cloud SQL instance"
  value       = google_sql_database_instance.main.private_ip_address
}

output "db_instance_ip" {
  description = "The private IP address of the Cloud SQL instance (public IP disabled for security)"
  value       = google_sql_database_instance.main.private_ip_address
  sensitive   = true
}

# Additional database outputs for scripts
output "db_instance_connection_name" {
  description = "The connection name of the database instance"
  value       = google_sql_database_instance.main.connection_name
}

output "db_name" {
  description = "The name of the database"
  value       = google_sql_database.default.name
}

output "db_user" {
  description = "The database user name"
  value       = google_sql_user.default.name
}

# Redis outputs
output "redis_host" {
  description = "The IP address of the Redis instance"
  value       = google_redis_instance.main.host
}

output "redis_port" {
  description = "The port of the Redis instance"
  value       = google_redis_instance.main.port
}

output "redis_auth_string" {
  description = "The AUTH string for the Redis instance"
  value       = google_redis_instance.main.auth_string
  sensitive   = true
}

# Cloud Run service URLs
output "backend_api_url" {
  description = "The URL of the backend API Cloud Run service"
  value       = google_cloud_run_service.backend_api.status[0].url
}

output "automation_engine_url" {
  description = "The URL of the automation engine Cloud Run service"
  value       = google_cloud_run_service.automation_engine.status[0].url
}

output "notification_service_url" {
  description = "The URL of the notification service Cloud Run service"
  value       = google_cloud_run_service.notification_service.status[0].url
}

# VPC outputs
output "vpc_connector_id" {
  description = "The ID of the VPC Access Connector"
  value       = google_vpc_access_connector.main.id
}

# Frontend outputs
output "frontend_bucket_name" {
  description = "The name of the frontend GCS bucket"
  value       = google_storage_bucket.frontend.name
}

output "frontend_load_balancer_ip" {
  description = "The IP address of the frontend load balancer"
  value       = google_compute_global_address.frontend.address
}

output "frontend_url" {
  description = "The URL of the frontend application"
  value       = "https://${var.domain_name}"
}

output "frontend_dns_nameservers" {
  description = "The nameservers for the DNS zone (if managed)"
  value       = var.enable_dns_zone ? one(google_dns_managed_zone.frontend[*].name_servers) : []
}

output "backend_api_custom_domain" {
  description = "The custom domain URL for the backend API"
  value       = "https://api.${var.domain_name}"
}

output "backend_api_load_balancer_ip" {
  description = "The IP address of the backend API load balancer"
  value       = google_compute_global_address.backend_api.address
}