terraform {
  backend "gcs" {
    bucket = "the-academy-sync-claude-tfstate"
    prefix = "the-academy-sync/state"    // This will store the state under gs://the-academy-sync-claude-tfstate/the-academy-sync/state
  }
}
