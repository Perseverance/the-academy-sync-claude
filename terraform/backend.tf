terraform {
  backend "gcs" {
    bucket = "the-academy-sync-claude-tfstate"
    prefix = "tf-state"
  }
}
