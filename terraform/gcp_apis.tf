resource "google_project_service" "sqladmin" {
  project                    = var.project_id
  service                    = "sqladmin.googleapis.com"
  disable_dependent_services = true
  disable_on_destroy         = true
}

resource "google_project_service" "secretmanager" {
  project                    = var.project_id
  service                    = "secretmanager.googleapis.com"
  disable_dependent_services = true
  disable_on_destroy         = true
}

resource "google_project_service" "run" {
  project                    = var.project_id
  service                    = "run.googleapis.com"
  disable_dependent_services = true
  disable_on_destroy         = true
}

resource "google_project_service" "vpcaccess" {
  project                    = var.project_id
  service                    = "vpcaccess.googleapis.com"
  disable_dependent_services = true
  disable_on_destroy         = true
}

resource "google_project_service" "compute" {
  project                    = var.project_id
  service                    = "compute.googleapis.com"
  disable_dependent_services = true
  disable_on_destroy         = true
}

resource "google_project_service" "redis" {
  project                    = var.project_id
  service                    = "redis.googleapis.com"
  disable_dependent_services = true
  disable_on_destroy         = true
}

resource "google_project_service" "servicenetworking" {
  project                    = var.project_id
  service                    = "servicenetworking.googleapis.com"
  disable_dependent_services = true
  disable_on_destroy         = true
}

resource "google_project_service" "iam" {
  project                    = var.project_id
  service                    = "iam.googleapis.com"
  disable_dependent_services = true
  disable_on_destroy         = true
}

resource "google_project_service" "dns" {
  project                    = var.project_id
  service                    = "dns.googleapis.com"
  disable_dependent_services = true
  disable_on_destroy         = true
}

resource "google_project_service" "cloudscheduler" {
  project                    = var.project_id
  service                    = "cloudscheduler.googleapis.com"
  disable_dependent_services = true
  disable_on_destroy         = true
}
