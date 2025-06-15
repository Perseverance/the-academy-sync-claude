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
    value = "203.0.113.0/24" # Replace with your actual development IP range
  }
]
