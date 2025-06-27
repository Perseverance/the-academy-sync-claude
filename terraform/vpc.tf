# VPC Access Connector for Cloud Run services to access private resources
resource "google_vpc_access_connector" "connector" {
  name          = "${var.environment}-vpc-connector"
  project       = var.project_id
  region        = var.region
  network       = "default"
  ip_cidr_range = var.vpc_connector_cidr

  # Machine type for the connector
  machine_type = var.vpc_connector_machine_type

  # Min and max instances for the connector
  min_instances = var.vpc_connector_min_instances
  max_instances = var.vpc_connector_max_instances

  depends_on = [
    google_project_service.vpcaccess,
    google_project_service.compute
  ]
}

output "vpc_connector_id" {
  description = "The ID of the VPC connector"
  value       = google_vpc_access_connector.connector.id
}