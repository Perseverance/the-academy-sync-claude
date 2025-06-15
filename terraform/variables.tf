variable "project_id" {
  description = "The GCP project ID to deploy resources into."
  type        = string
}

variable "region" {
  description = "The GCP region where resources will be deployed."
  type        = string
  default     = "us-central1" // Example default, can be overridden
}

variable "machine_type" {
  description = "The machine type for compute instances."
  type        = string
  default     = "e2-medium" // Example default, can be overridden
}

variable "instance_count" {
  description = "The number of instances to create."
  type        = number
  default     = 1 // Example default, can be overridden
}
