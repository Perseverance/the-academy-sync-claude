# Staging Environment Configuration

project_id = "the-academy-sync-sdlc-test"
region     = "europe-central2"

db_tier                           = "db-custom-1-3840"
db_disk_size                      = 10
db_availability_type              = "ZONAL"
db_backups_enabled                = false
db_point_in_time_recovery_enabled = false
db_deletion_protection            = false

environment = "staging"

# For staging, we can be more permissive but still avoid 0.0.0.0/0
authorized_networks = [
  {
    name  = "Development Network"
    value = "203.0.113.0/24" 
  },
  {
    name  = "Development Network"
    value = "78.128.35.0/24" 
  }
]

# Redis configuration for staging
redis_tier           = "BASIC"
redis_memory_size_gb = 1

# Container image URLs - Build and push images first, then update these
# Example build command: docker build --build-arg SERVICE_NAME=backend-api -t gcr.io/the-academy-sync-sdlc-test/backend-api:latest .
# Example push command: docker push gcr.io/the-academy-sync-sdlc-test/backend-api:latest
backend_api_image_url          = "gcr.io/the-academy-sync-sdlc-test/backend-api:staging"
automation_engine_image_url    = "gcr.io/the-academy-sync-sdlc-test/automation-engine:staging"
notification_service_image_url = "gcr.io/the-academy-sync-sdlc-test/notification-service:staging"

# Service configurations for staging (cost-optimized)
backend_api_cpu            = "1"
backend_api_memory         = "512Mi"
backend_api_min_instances  = 0  # Allow scale to zero in staging
backend_api_max_instances  = 3

automation_engine_cpu            = "1"
automation_engine_memory         = "1Gi"
automation_engine_min_instances  = 0  # Allow scale to zero in staging
automation_engine_max_instances  = 2
automation_engine_max_workers    = 10

notification_service_cpu            = "0.5"
notification_service_memory         = "256Mi"
notification_service_min_instances  = 0  # Allow scale to zero in staging
notification_service_max_instances  = 2

# VPC Connector for staging (smaller configuration)
vpc_connector_machine_type  = "f1-micro"
vpc_connector_min_instances = 2
vpc_connector_max_instances = 3

# Logging
log_level = "INFO"
