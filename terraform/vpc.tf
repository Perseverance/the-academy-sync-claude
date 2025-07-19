# VPC and Networking Resources

# VPC for private resources
resource "google_compute_network" "main" {
  name                    = "${var.environment}-vpc"
  project                 = var.project_id
  auto_create_subnetworks = false

  depends_on = [google_project_service.compute]
}

# VPC Access Connector for Cloud Run
resource "google_vpc_access_connector" "main" {
  name          = "${var.environment}-vpc-connector"
  project       = var.project_id
  region        = var.region
  network       = google_compute_network.main.id
  ip_cidr_range = "10.10.0.0/28"

  # Machine type for the connector
  machine_type = "e2-micro"

  # Min and max instances
  min_instances = 2
  max_instances = 3

  depends_on = [google_project_service.vpcaccess]
}

# Private IP address range for services
resource "google_compute_global_address" "private_ip_address" {
  name          = "${var.environment}-private-ip-address"
  project       = var.project_id
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = google_compute_network.main.id
}

# Private VPC connection
resource "google_service_networking_connection" "private_vpc_connection" {
  network                 = google_compute_network.main.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_address.name]

  depends_on = [google_project_service.servicenetworking]
}