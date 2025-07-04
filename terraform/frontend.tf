# Frontend Static Site Infrastructure

# DNS Managed Zone (optional - only created if enable_dns_zone is true)
resource "google_dns_managed_zone" "frontend" {
  count       = var.enable_dns_zone ? 1 : 0
  name        = "${var.environment}-frontend-dns-zone"
  dns_name    = "${var.domain_name}."
  description = "DNS zone for ${var.domain_name}"
  project     = var.project_id

  depends_on = [google_project_service.dns]
}

# GCS Bucket for static content
resource "google_storage_bucket" "frontend" {
  name     = "${var.project_id}-frontend-${var.environment}"
  location = var.region
  project  = var.project_id

  force_destroy = true

  uniform_bucket_level_access = true

  website {
    main_page_suffix = "index.html"
    not_found_page   = "404.html"
  }

  cors {
    origin          = ["*"]
    method          = ["GET", "HEAD", "OPTIONS"]
    response_header = ["*"]
    max_age_seconds = 3600
  }

  lifecycle_rule {
    condition {
      age = 30
    }
    action {
      type = "Delete"
    }
  }

  depends_on = [google_project_service.compute]
}

# Make bucket content publicly accessible
resource "google_storage_bucket_iam_member" "frontend_public_access" {
  bucket = google_storage_bucket.frontend.name
  role   = "roles/storage.objectViewer"
  member = "allUsers"
}

# Static IP for the load balancer
resource "google_compute_global_address" "frontend" {
  name    = "${var.environment}-frontend-ip"
  project = var.project_id

  depends_on = [google_project_service.compute]
}

# SSL Certificate
resource "google_compute_managed_ssl_certificate" "frontend" {
  name    = "${var.environment}-frontend-ssl-cert"
  project = var.project_id

  managed {
    domains = [var.domain_name]
  }

  depends_on = [google_project_service.compute]
}

# Backend bucket for Cloud CDN
resource "google_compute_backend_bucket" "frontend" {
  name        = "${var.environment}-frontend-backend-bucket"
  bucket_name = google_storage_bucket.frontend.name
  project     = var.project_id

  enable_cdn = true

  cdn_policy {
    cache_mode        = "CACHE_ALL_STATIC"
    client_ttl        = 3600
    default_ttl       = 3600
    max_ttl           = 86400
    negative_caching  = true
    serve_while_stale = 86400

    negative_caching_policy {
      code = 404
      ttl  = 300
    }
  }

  depends_on = [google_project_service.compute]
}

# URL Map
resource "google_compute_url_map" "frontend" {
  name            = "${var.environment}-frontend-url-map"
  default_service = google_compute_backend_bucket.frontend.id
  project         = var.project_id

  depends_on = [google_project_service.compute]
}

# HTTPS Proxy
resource "google_compute_target_https_proxy" "frontend" {
  name    = "${var.environment}-frontend-https-proxy"
  url_map = google_compute_url_map.frontend.id
  project = var.project_id

  ssl_certificates = [google_compute_managed_ssl_certificate.frontend.id]

  depends_on = [google_project_service.compute]
}

# Global Forwarding Rule
resource "google_compute_global_forwarding_rule" "frontend" {
  name                  = "${var.environment}-frontend-https-rule"
  ip_protocol           = "TCP"
  load_balancing_scheme = "EXTERNAL"
  port_range            = "443"
  target                = google_compute_target_https_proxy.frontend.id
  ip_address            = google_compute_global_address.frontend.id
  project               = var.project_id

  depends_on = [google_project_service.compute]
}

# HTTP to HTTPS redirect
resource "google_compute_url_map" "frontend_http_redirect" {
  name    = "${var.environment}-frontend-http-redirect"
  project = var.project_id

  default_url_redirect {
    https_redirect         = true
    redirect_response_code = "MOVED_PERMANENTLY_DEFAULT"
    strip_query            = false
  }

  depends_on = [google_project_service.compute]
}

resource "google_compute_target_http_proxy" "frontend_http" {
  name    = "${var.environment}-frontend-http-proxy"
  url_map = google_compute_url_map.frontend_http_redirect.id
  project = var.project_id

  depends_on = [google_project_service.compute]
}

resource "google_compute_global_forwarding_rule" "frontend_http" {
  name                  = "${var.environment}-frontend-http-rule"
  ip_protocol           = "TCP"
  load_balancing_scheme = "EXTERNAL"
  port_range            = "80"
  target                = google_compute_target_http_proxy.frontend_http.id
  ip_address            = google_compute_global_address.frontend.id
  project               = var.project_id

  depends_on = [google_project_service.compute]
}

# DNS A Record (only created if DNS zone is managed)
resource "google_dns_record_set" "frontend_a" {
  count = var.enable_dns_zone ? 1 : 0

  name         = var.domain_name != google_dns_managed_zone.frontend[0].dns_name ? "${var.domain_name}." : google_dns_managed_zone.frontend[0].dns_name
  type         = "A"
  ttl          = 300
  managed_zone = google_dns_managed_zone.frontend[0].name
  project      = var.project_id

  rrdatas = [google_compute_global_address.frontend.address]

  depends_on = [google_dns_managed_zone.frontend]
}

# DNS A Record for Backend API (only created if DNS zone is managed)
resource "google_dns_record_set" "backend_api_a" {
  count = var.enable_dns_zone ? 1 : 0

  name         = "api.${var.domain_name}."
  type         = "A"
  ttl          = 300
  managed_zone = google_dns_managed_zone.frontend[0].name
  project      = var.project_id

  # Point to the backend API load balancer IP
  rrdatas = [google_compute_global_address.backend_api.address]

  depends_on = [google_dns_managed_zone.frontend, google_compute_global_address.backend_api]
}