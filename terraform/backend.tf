terraform {
  backend "gcs" {
    bucket  = "your-gcs-bucket-name-here"  // TODO: Replace with your actual GCS bucket name
    prefix  = "tf-state/${terraform.workspace}"     // This will store the state under gs://<bucket_name>/tf-state/<workspace_name>
  }
}
