output "service_url" {
  description = "The URL of the deployed Cloud Run service"
  value       = google_cloud_run_v2_service.service.uri
}

output "service_name" {
  description = "The name of the Cloud Run service"
  value       = google_cloud_run_v2_service.service.name
}

output "service_account_email" {
  description = "The email of the service account created for this service"
  value       = google_service_account.service_account.email
}

output "service_id" {
  description = "The ID of the Cloud Run service"
  value       = google_cloud_run_v2_service.service.id
}