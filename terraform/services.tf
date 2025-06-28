# Backend API Service
module "backend_api" {
  source = "./modules/cloud-run-service"

  service_name = "backend-api"
  project_id   = var.project_id
  region       = var.region
  environment  = var.environment
  image_url    = var.backend_api_image_url

  # Environment variables
  env_vars = {
    APP_ENV         = var.environment
    GO_ENV          = var.environment
    # PORT is automatically set by Cloud Run, don't set it here
    LOG_LEVEL       = var.log_level
    GCP_PROJECT_ID  = var.project_id
    FAIL_FAST_ENABLED = "false"  # Disable for initial deployment
    # These will be replaced with actual URLs after deployment
    BASE_URL        = "https://staging-backend-api-placeholder.a.run.app"
    FRONTEND_URL    = "https://staging.example.com"
    # SMTP configuration
    SMTP_HOST       = "smtp.gmail.com"
    SMTP_PORT       = "587"
  }

  # Secrets from Secret Manager
  secrets = {
    DATABASE_URL         = "${var.environment}-database-url"
    REDIS_URL            = "${var.environment}-redis-url"
    JWT_SECRET           = "${var.environment}-jwt-secret"
    ENCRYPTION_SECRET    = "${var.environment}-encryption-secret"
    GOOGLE_CLIENT_ID     = "${var.environment}-google-client-id"
    GOOGLE_CLIENT_SECRET = "${var.environment}-google-client-secret"
    STRAVA_CLIENT_ID     = "${var.environment}-strava-client-id"
    STRAVA_CLIENT_SECRET = "${var.environment}-strava-client-secret"
    SMTP_USERNAME        = "${var.environment}-smtp-username"
    SMTP_PASSWORD        = "${var.environment}-smtp-password"
    FROM_EMAIL           = "${var.environment}-from-email"
  }

  # Required secrets for IAM permissions
  required_secrets = [
    "${var.environment}-database-url",
    "${var.environment}-redis-url",
    "${var.environment}-jwt-secret",
    "${var.environment}-encryption-secret",
    "${var.environment}-google-client-id",
    "${var.environment}-google-client-secret",
    "${var.environment}-strava-client-id",
    "${var.environment}-strava-client-secret",
    "${var.environment}-smtp-username",
    "${var.environment}-smtp-password",
    "${var.environment}-from-email"
  ]

  # Resource configuration
  cpu_limit    = var.backend_api_cpu
  memory_limit = var.backend_api_memory

  # VPC connector for private access to Cloud SQL and Redis
  vpc_connector_id = google_vpc_access_connector.connector.id

  # Scaling configuration
  min_instances = var.backend_api_min_instances
  max_instances = var.backend_api_max_instances

  # Allow public access for the API
  allow_unauthenticated = true

  depends_on = [
    google_project_service.run,
    google_vpc_access_connector.connector
  ]
}

# Automation Engine Service
module "automation_engine" {
  source = "./modules/cloud-run-service"

  service_name = "automation-engine"
  project_id   = var.project_id
  region       = var.region
  environment  = var.environment
  image_url    = var.automation_engine_image_url

  # Environment variables
  env_vars = {
    APP_ENV           = var.environment
    GO_ENV            = var.environment
    LOG_LEVEL         = var.log_level
    GCP_PROJECT_ID    = var.project_id
    FAIL_FAST_ENABLED = "false"  # Disable for initial deployment
    MAX_WORKERS       = tostring(var.automation_engine_max_workers)
    # BASE_URL required by config validation even for internal services
    BASE_URL          = "https://staging-automation-engine.a.run.app"
  }

  # Secrets from Secret Manager
  secrets = {
    DATABASE_URL      = "${var.environment}-database-url"
    REDIS_URL         = "${var.environment}-redis-url"
    ENCRYPTION_SECRET = "${var.environment}-encryption-secret"
    JWT_SECRET        = "${var.environment}-jwt-secret"
  }

  # Required secrets for IAM permissions
  required_secrets = [
    "${var.environment}-database-url",
    "${var.environment}-redis-url",
    "${var.environment}-encryption-secret",
    "${var.environment}-jwt-secret"
  ]

  # Resource configuration
  cpu_limit    = var.automation_engine_cpu
  memory_limit = var.automation_engine_memory

  # VPC connector for private access to Cloud SQL and Redis
  vpc_connector_id = google_vpc_access_connector.connector.id

  # Scaling configuration
  min_instances = var.automation_engine_min_instances
  max_instances = var.automation_engine_max_instances

  # This is an internal service, no public access
  allow_unauthenticated = false

  depends_on = [
    google_project_service.run,
    google_vpc_access_connector.connector
  ]
}

# Notification Service
module "notification_service" {
  source = "./modules/cloud-run-service"

  service_name = "notification-service"
  project_id   = var.project_id
  region       = var.region
  environment  = var.environment
  image_url    = var.notification_service_image_url

  # Environment variables
  env_vars = {
    APP_ENV           = var.environment
    GO_ENV            = var.environment
    LOG_LEVEL         = var.log_level
    GCP_PROJECT_ID    = var.project_id
    FAIL_FAST_ENABLED = "false" # Notification service can run without DB
    SMTP_HOST         = "smtp.gmail.com"
    SMTP_PORT         = "587"
    # BASE_URL required by config validation even for internal services
    BASE_URL          = "https://staging-notification-service.a.run.app"
  }

  # Secrets from Secret Manager
  secrets = {
    DATABASE_URL      = "${var.environment}-database-url" # Optional for notification service
    SMTP_USERNAME     = "${var.environment}-smtp-username"
    SMTP_PASSWORD     = "${var.environment}-smtp-password"
    FROM_EMAIL        = "${var.environment}-from-email"
    JWT_SECRET        = "${var.environment}-jwt-secret"  # Required by config validation
    ENCRYPTION_SECRET = "${var.environment}-encryption-secret"  # Required by config validation
  }

  # Required secrets for IAM permissions
  required_secrets = [
    "${var.environment}-database-url",
    "${var.environment}-smtp-username",
    "${var.environment}-smtp-password",
    "${var.environment}-from-email",
    "${var.environment}-jwt-secret",
    "${var.environment}-encryption-secret"
  ]

  # Resource configuration
  cpu_limit    = var.notification_service_cpu
  memory_limit = var.notification_service_memory

  # VPC connector for private access to Cloud SQL
  vpc_connector_id = google_vpc_access_connector.connector.id

  # Scaling configuration
  min_instances = var.notification_service_min_instances
  max_instances = var.notification_service_max_instances

  # This is an internal service, no public access
  allow_unauthenticated = false

  # For CPU < 1, we must set concurrency to 1
  concurrency = 1

  depends_on = [
    google_project_service.run,
    google_vpc_access_connector.connector
  ]
}

# Output the service URLs
output "backend_api_url" {
  description = "The URL of the backend API service"
  value       = module.backend_api.service_url
}

output "automation_engine_url" {
  description = "The URL of the automation engine service"
  value       = module.automation_engine.service_url
}

output "notification_service_url" {
  description = "The URL of the notification service"
  value       = module.notification_service.service_url
}