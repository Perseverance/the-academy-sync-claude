# Backend API Load Balancer Configuration

# NEG (Network Endpoint Group) for Cloud Run service
resource "google_compute_region_network_endpoint_group" "backend_api_neg" {
  name                  = "${var.environment}-backend-api-neg"
  network_endpoint_type = "SERVERLESS"
  region                = var.region
  project               = var.project_id

  cloud_run {
    service = google_cloud_run_service.backend_api.name
  }

  depends_on = [google_project_service.compute]
}

# Backend service
resource "google_compute_backend_service" "backend_api" {
  name        = "${var.environment}-backend-api-backend"
  protocol    = "HTTPS"
  timeout_sec = 30
  project     = var.project_id

  # No port_name for serverless NEGs
  load_balancing_scheme = "EXTERNAL"

  backend {
    group = google_compute_region_network_endpoint_group.backend_api_neg.id
  }

  depends_on = [google_project_service.compute]
}

# URL map
resource "google_compute_url_map" "backend_api" {
  name            = "${var.environment}-backend-api-url-map"
  default_service = google_compute_backend_service.backend_api.id
  project         = var.project_id

  depends_on = [google_project_service.compute]
}

# SSL certificate for backend API
resource "google_compute_managed_ssl_certificate" "backend_api" {
  name    = "${var.environment}-backend-api-ssl-cert"
  project = var.project_id

  managed {
    domains = ["api.${var.domain_name}"]
  }

  depends_on = [google_project_service.compute]
}

# HTTPS proxy
resource "google_compute_target_https_proxy" "backend_api" {
  name             = "${var.environment}-backend-api-https-proxy"
  url_map          = google_compute_url_map.backend_api.id
  ssl_certificates = [google_compute_managed_ssl_certificate.backend_api.id]
  project          = var.project_id

  depends_on = [google_project_service.compute]
}

# Global IP address for backend API
resource "google_compute_global_address" "backend_api" {
  name    = "${var.environment}-backend-api-ip"
  project = var.project_id

  depends_on = [google_project_service.compute]
}

# Global forwarding rule
resource "google_compute_global_forwarding_rule" "backend_api_https" {
  name                  = "${var.environment}-backend-api-https-rule"
  ip_protocol           = "TCP"
  load_balancing_scheme = "EXTERNAL"
  port_range            = "443"
  target                = google_compute_target_https_proxy.backend_api.id
  ip_address            = google_compute_global_address.backend_api.id
  project               = var.project_id

  depends_on = [google_project_service.compute]
}

# HTTP to HTTPS redirect
resource "google_compute_url_map" "backend_api_http_redirect" {
  name    = "${var.environment}-backend-api-http-redirect"
  project = var.project_id

  default_url_redirect {
    https_redirect         = true
    redirect_response_code = "MOVED_PERMANENTLY_DEFAULT"
    strip_query            = false
  }

  depends_on = [google_project_service.compute]
}

resource "google_compute_target_http_proxy" "backend_api_http" {
  name    = "${var.environment}-backend-api-http-proxy"
  url_map = google_compute_url_map.backend_api_http_redirect.id
  project = var.project_id

  depends_on = [google_project_service.compute]
}

resource "google_compute_global_forwarding_rule" "backend_api_http" {
  name                  = "${var.environment}-backend-api-http-rule"
  ip_protocol           = "TCP"
  load_balancing_scheme = "EXTERNAL"
  port_range            = "80"
  target                = google_compute_target_http_proxy.backend_api_http.id
  ip_address            = google_compute_global_address.backend_api.id
  project               = var.project_id

  depends_on = [google_project_service.compute]
}