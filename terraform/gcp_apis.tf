resource "google_project_service" "sqladmin" {
  project                    = var.project_id
  service                    = "sqladmin.googleapis.com"
  disable_dependent_services = true
}

resource "google_project_service" "secretmanager" {
  project                    = var.project_id
  service                    = "secretmanager.googleapis.com"
  disable_dependent_services = true
}

resource "google_project_service" "redis" {
  project                    = var.project_id
  service                    = "redis.googleapis.com"
  disable_dependent_services = true
}

resource "google_project_service" "servicenetworking" {
  project                    = var.project_id
  service                    = "servicenetworking.googleapis.com"
  disable_dependent_services = true
}

resource "google_project_service" "compute" {
  project                    = var.project_id
  service                    = "compute.googleapis.com"
  disable_dependent_services = true
}
