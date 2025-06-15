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
