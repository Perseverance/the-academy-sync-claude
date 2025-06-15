terraform {
  backend "gcs" {
    bucket  = "your-gcs-bucket-name-here"  // TODO: Replace with your actual GCS bucket name
    prefix  = "the-academy-sync/state"     // This will store the state under gs://your-gcs-bucket-name-here/the-academy-sync/state
  }
}
