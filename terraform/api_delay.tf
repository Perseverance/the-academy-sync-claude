# Add a delay after API enablement to ensure they're fully propagated
resource "time_sleep" "wait_for_apis" {
  depends_on = [
    google_project_service.compute,
    google_project_service.sqladmin,
    google_project_service.secretmanager,
    google_project_service.run,
    google_project_service.vpcaccess,
    google_project_service.redis,
    google_project_service.servicenetworking,
    google_project_service.iam,
    google_project_service.dns
  ]

  create_duration = "30s"
}

# Update other resources to depend on this delay
# For example, in your other files, add:
# depends_on = [time_sleep.wait_for_apis]