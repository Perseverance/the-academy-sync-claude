# Cloud Run Service Module

This Terraform module creates a secure, standardized Cloud Run service with the following features:

- Dedicated service account with least-privilege access
- Secret Manager integration for sensitive environment variables
- VPC connector support for private network access
- Configurable scaling and resource limits
- Environment-specific naming conventions

## Usage

```hcl
module "backend_api" {
  source = "./modules/cloud-run-service"

  service_name = "backend-api"
  project_id   = var.project_id
  region       = var.region
  environment  = var.environment
  image_url    = var.backend_api_image_url

  # Plain environment variables
  env_vars = {
    APP_ENV      = var.environment
    PORT         = "8080"
    LOG_LEVEL    = "INFO"
    BASE_URL     = "https://api.example.com"
    FRONTEND_URL = "https://app.example.com"
  }

  # Secret environment variables (from Secret Manager)
  secrets = {
    DATABASE_URL        = "${var.environment}-database-url"
    REDIS_URL           = "${var.environment}-redis-url"
    JWT_SECRET          = "${var.environment}-jwt-secret"
    ENCRYPTION_SECRET   = "${var.environment}-encryption-secret"
    GOOGLE_CLIENT_ID    = "${var.environment}-google-client-id"
    GOOGLE_CLIENT_SECRET = "${var.environment}-google-client-secret"
  }

  # List of secrets this service needs access to
  required_secrets = [
    "${var.environment}-database-url",
    "${var.environment}-redis-url",
    "${var.environment}-jwt-secret",
    "${var.environment}-encryption-secret",
    "${var.environment}-google-client-id",
    "${var.environment}-google-client-secret"
  ]

  # Resource configuration
  cpu_limit    = "1"
  memory_limit = "1Gi"

  # VPC connector for private network access
  vpc_connector_id = google_vpc_access_connector.connector.id

  # Scaling configuration
  min_instances = 1
  max_instances = 10

  # Allow public access
  allow_unauthenticated = true
}
```

## Input Variables

| Variable | Description | Type | Default |
|----------|-------------|------|---------|
| `service_name` | The name of the Cloud Run service | `string` | - |
| `project_id` | The GCP project ID | `string` | - |
| `region` | The GCP region for the Cloud Run service | `string` | - |
| `environment` | The environment name (e.g., staging, prod) | `string` | - |
| `image_url` | The container image URL to deploy | `string` | - |
| `env_vars` | Map of environment variables to set in the container | `map(string)` | `{}` |
| `secrets` | Map of secret environment variables from Secret Manager | `map(string)` | `{}` |
| `required_secrets` | List of secret IDs that this service needs access to | `list(string)` | `[]` |
| `cpu_limit` | CPU limit for the service | `string` | `"1"` |
| `memory_limit` | Memory limit for the service | `string` | `"512Mi"` |
| `vpc_connector_id` | The ID of the VPC connector for private network access | `string` | `""` |
| `min_instances` | Minimum number of instances | `number` | `0` |
| `max_instances` | Maximum number of instances | `number` | `100` |
| `timeout_seconds` | Request timeout in seconds | `number` | `300` |
| `concurrency` | Maximum concurrent requests per instance | `number` | `80` |
| `allow_unauthenticated` | Whether to allow unauthenticated access | `bool` | `false` |

## Outputs

| Output | Description |
|--------|-------------|
| `service_url` | The URL of the deployed Cloud Run service |
| `service_name` | The name of the Cloud Run service |
| `service_account_email` | The email of the service account created for this service |
| `service_id` | The ID of the Cloud Run service |

## Security

This module implements several security best practices:

1. **Dedicated Service Account**: Each service runs with its own service account, following the principle of least privilege.

2. **Secret Access Control**: Services only get access to the specific secrets they need, not all secrets in the project.

3. **VPC Connector**: When configured, services use private IP ranges to communicate with other resources like Cloud SQL and Redis.

4. **No Default Public Access**: Services are private by default unless explicitly configured with `allow_unauthenticated = true`.

## Notes

- The module automatically prefixes service names with the environment (e.g., `staging-backend-api`)
- Secret environment variables reference Secret Manager secrets and are automatically injected at runtime
- The VPC connector is optional but recommended for production deployments that need to access private resources