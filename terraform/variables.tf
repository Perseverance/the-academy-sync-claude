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
