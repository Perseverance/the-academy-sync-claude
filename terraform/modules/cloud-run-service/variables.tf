variable "service_name" {
  description = "The name of the Cloud Run service"
  type        = string
}

variable "project_id" {
  description = "The GCP project ID"
  type        = string
}

variable "region" {
  description = "The GCP region for the Cloud Run service"
  type        = string
}

variable "environment" {
  description = "The environment name (e.g., staging, prod)"
  type        = string
}

variable "image_url" {
  description = "The container image URL to deploy"
  type        = string
}

variable "env_vars" {
  description = "Map of environment variables to set in the container"
  type        = map(string)
  default     = {}
}

variable "secrets" {
  description = "Map of secret environment variables from Secret Manager. Key is env var name, value is secret ID"
  type        = map(string)
  default     = {}
}

variable "required_secrets" {
  description = "List of secret IDs that this service needs access to"
  type        = list(string)
  default     = []
}

variable "cpu_limit" {
  description = "CPU limit for the service (e.g., '1', '2', '0.5')"
  type        = string
  default     = "1"
}

variable "memory_limit" {
  description = "Memory limit for the service (e.g., '512Mi', '1Gi', '2Gi')"
  type        = string
  default     = "512Mi"
}

variable "vpc_connector_id" {
  description = "The ID of the VPC connector for private network access"
  type        = string
  default     = ""
}

variable "min_instances" {
  description = "Minimum number of instances (0 allows scale to zero)"
  type        = number
  default     = 0
}

variable "max_instances" {
  description = "Maximum number of instances"
  type        = number
  default     = 100
}

variable "timeout_seconds" {
  description = "Request timeout in seconds"
  type        = number
  default     = 300
}

variable "concurrency" {
  description = "Maximum number of concurrent requests per instance"
  type        = number
  default     = 80
}

variable "allow_unauthenticated" {
  description = "Whether to allow unauthenticated access to the service"
  type        = bool
  default     = false
}

variable "cloud_sql_connection_name" {
  description = "The Cloud SQL instance connection name (e.g., project:region:instance)"
  type        = string
  default     = ""
}