# Production Environment Configuration

project_id = "the-academy-sync-sdlc-test"
region     = "europe-central2"

db_tier                           = "db-n2-standard-2"
db_disk_size                      = 25
db_availability_type              = "REGIONAL"
db_backups_enabled                = true
db_point_in_time_recovery_enabled = true
db_deletion_protection            = true

environment = "prod"

# Authorized networks - should be restricted to your actual IP ranges
# Example: VPN, office networks, or specific application servers
authorized_networks = [
  {
    name  = "Office Network"
    value = "203.0.113.0/24" # Replace with your actual IP range
  },
  {
    name  = "VPN Network"
    value = "198.51.100.0/24" # Replace with your actual VPN range
  }
]

# Redis configuration for production
redis_tier           = "STANDARD_HA"
redis_memory_size_gb = 1

# Container image URLs - Build and push images first, then update these
# Example build command: docker build --build-arg SERVICE_NAME=backend-api -t gcr.io/the-academy-sync-sdlc-test/backend-api:latest .
# Example push command: docker push gcr.io/the-academy-sync-sdlc-test/backend-api:latest
backend_api_image_url          = "gcr.io/the-academy-sync-sdlc-test/backend-api:placeholder"
automation_engine_image_url    = "gcr.io/the-academy-sync-sdlc-test/automation-engine:placeholder"
notification_service_image_url = "gcr.io/the-academy-sync-sdlc-test/notification-service:placeholder"

# Service configurations for production (performance-optimized)
backend_api_cpu            = "2"
backend_api_memory         = "1Gi"
backend_api_min_instances  = 1  # Always have at least one instance running
backend_api_max_instances  = 20

automation_engine_cpu            = "4"
automation_engine_memory         = "2Gi"
automation_engine_min_instances  = 1  # Always have at least one instance running
automation_engine_max_instances  = 10
automation_engine_max_workers    = 50

notification_service_cpu            = "1"
notification_service_memory         = "512Mi"
notification_service_min_instances  = 1  # Always have at least one instance running
notification_service_max_instances  = 5

# VPC Connector for production (larger configuration)
vpc_connector_machine_type  = "e2-standard-4"
vpc_connector_min_instances = 3
vpc_connector_max_instances = 10

# Logging
log_level = "WARNING"