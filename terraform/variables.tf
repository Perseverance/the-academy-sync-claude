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
