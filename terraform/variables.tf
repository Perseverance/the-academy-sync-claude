variable "project_id" {
  description = "The GCP project ID to deploy resources into."
  type        = string
}

variable "region" {
  description = "The GCP region where resources will be deployed."
  type        = string
  default     = "europe-central2"
}

variable "db_tier" {
  description = "The machine type for the database instance."
  type        = string
}

variable "db_disk_size" {
  description = "The disk size for the database instance in GB."
  type        = number
}

variable "db_availability_type" {
  description = "The availability type for the database instance. Either ZONAL or REGIONAL."
  type        = string
}

variable "db_backups_enabled" {
  description = "Whether automated backups are enabled."
  type        = bool
}

variable "db_point_in_time_recovery_enabled" {
  description = "Whether point-in-time recovery is enabled."
  type        = bool
}

variable "db_deletion_protection" {
  description = "Whether deletion protection is enabled."
  type        = bool
}

variable "environment" {
  description = "The environment name (e.g., staging, prod)."
  type        = string
}

variable "authorized_networks" {
  description = "List of authorized networks that can connect to the database."
  type = list(object({
    name  = string
    value = string
  }))
  default = []
}

variable "redis_tier" {
  description = "The tier of the Redis instance. Either BASIC or STANDARD_HA."
  type        = string
}

variable "redis_memory_size_gb" {
  description = "The memory size of the Redis instance in GB."
  type        = number
}

# VPC Connector configuration
variable "vpc_connector_cidr" {
  description = "The CIDR range for the VPC connector"
  type        = string
  default     = "10.8.0.0/28"
}

variable "vpc_connector_machine_type" {
  description = "Machine type for VPC connector instances"
  type        = string
  default     = "e2-micro"
}

variable "vpc_connector_min_instances" {
  description = "Minimum instances for VPC connector"
  type        = number
  default     = 2
}

variable "vpc_connector_max_instances" {
  description = "Maximum instances for VPC connector"
  type        = number
  default     = 10
}

# Container image URLs
variable "backend_api_image_url" {
  description = "Container image URL for backend-api service"
  type        = string
}

variable "automation_engine_image_url" {
  description = "Container image URL for automation-engine service"
  type        = string
}

variable "notification_service_image_url" {
  description = "Container image URL for notification-service service"
  type        = string
}

# Service resource configurations
variable "backend_api_cpu" {
  description = "CPU limit for backend-api service"
  type        = string
  default     = "1"
}

variable "backend_api_memory" {
  description = "Memory limit for backend-api service"
  type        = string
  default     = "512Mi"
}

variable "backend_api_min_instances" {
  description = "Minimum instances for backend-api service"
  type        = number
  default     = 1
}

variable "backend_api_max_instances" {
  description = "Maximum instances for backend-api service"
  type        = number
  default     = 10
}

variable "automation_engine_cpu" {
  description = "CPU limit for automation-engine service"
  type        = string
  default     = "2"
}

variable "automation_engine_memory" {
  description = "Memory limit for automation-engine service"
  type        = string
  default     = "1Gi"
}

variable "automation_engine_min_instances" {
  description = "Minimum instances for automation-engine service"
  type        = number
  default     = 1
}

variable "automation_engine_max_instances" {
  description = "Maximum instances for automation-engine service"
  type        = number
  default     = 5
}

variable "automation_engine_max_workers" {
  description = "Maximum worker threads for automation engine"
  type        = number
  default     = 20
}

variable "notification_service_cpu" {
  description = "CPU limit for notification-service"
  type        = string
  default     = "0.5"
}

variable "notification_service_memory" {
  description = "Memory limit for notification-service"
  type        = string
  default     = "256Mi"
}

variable "notification_service_min_instances" {
  description = "Minimum instances for notification-service"
  type        = number
  default     = 0
}

variable "notification_service_max_instances" {
  description = "Maximum instances for notification-service"
  type        = number
  default     = 5
}

# Common service configuration
variable "log_level" {
  description = "Log level for services"
  type        = string
  default     = "INFO"
}
