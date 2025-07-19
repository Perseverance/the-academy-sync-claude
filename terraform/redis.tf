# Redis (Memorystore) Resources

# Redis instance
resource "google_redis_instance" "main" {
  name           = "${var.environment}-redis-instance"
  project        = var.project_id
  region         = var.region
  location_id    = "${var.region}-b" # Changed from zone-a to zone-b
  tier           = "BASIC"           # Using BASIC tier for better availability
  memory_size_gb = var.redis_memory_size_gb
  redis_version  = "REDIS_7_0"
  display_name   = "${var.environment} Redis Instance"

  # Enable AUTH
  auth_enabled = true

  # Enable transit encryption (TLS)
  transit_encryption_mode = "SERVER_AUTHENTICATION"

  # Connect to our VPC
  authorized_network = google_compute_network.main.id
  connect_mode       = "PRIVATE_SERVICE_ACCESS"

  # Maintenance window (Sundays at 4 AM)
  maintenance_policy {
    weekly_maintenance_window {
      day = "SUNDAY"
      start_time {
        hours   = 4
        minutes = 0
        seconds = 0
        nanos   = 0
      }
    }
  }

  depends_on = [
    google_service_networking_connection.private_vpc_connection,
    google_project_service.redis
  ]
}