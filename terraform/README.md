# Terraform Configuration for The Academy Sync

This directory contains the Terraform configuration for deploying The Academy Sync infrastructure.

## Remote State Backend (Google Cloud Storage)

This project is configured to use a Google Cloud Storage (GCS) bucket as a remote backend for storing the Terraform state file (`terraform.tfstate`). This allows for secure state management, consistency, and team collaboration.

### Prerequisites: Manual GCS Bucket Creation

Before you can initialize Terraform and use the remote backend, you **must manually create a GCS bucket** that will store the state file.

### Initial GCP Setup & Authentication

Before creating the GCS bucket or running any Terraform commands that interact with Google Cloud, ensure you have:

1.  **Authenticated with GCP:**
    *   For interactive use, run:
        ```sh
        gcloud auth login
        ```
        And follow the prompts to log in with your Google account.
    *   For non-interactive environments (like CI/CD pipelines), ensure the `GOOGLE_APPLICATION_CREDENTIALS` environment variable is set to the path of your service account key JSON file.

2.  **Enabled the Cloud Storage API:**
    The Google Cloud Storage API must be enabled for your project. You can enable it by running:
    ```sh
    gcloud services enable storage.googleapis.com --project=your-gcp-project-id
    ```
    Replace `your-gcp-project-id` with your actual GCP Project ID.

Once these steps are complete, you can proceed to create the GCS bucket.

**Recommended GCS Bucket Configuration:**

*   **Unique Name:** Choose a globally unique name for your bucket (e.g., `your-unique-project-name-tfstate`).
*   **Location:** Choose a location for your bucket (e.g., `europe-central2`).
*   **Uniform Bucket-Level Access:** Enable this for consistent permission management.
*   **Public Access Prevention:** Ensure "Enforce public access prevention" is ON.
*   **Object Versioning:** Enable object versioning to protect against accidental state deletion or corruption.

**Example `gcloud` command to create such a bucket:**

```sh
gcloud storage buckets create gs://your-unique-project-name-tfstate \
    --project=your-gcp-project-id \
    --location=europe-central2 \
    --uniform-bucket-level-access \
    --public-access-prevention
gcloud storage buckets update gs://your-unique-project-name-tfstate --versioning
```

Replace `your-unique-project-name-tfstate` and `your-gcp-project-id` with your actual values.

### Updating `backend.tf`

Once the bucket is created, you need to update the `terraform/backend.tf` file:

```terraform
terraform {
  backend "gcs" {
    bucket  = "your-gcs-bucket-name-here"  // TODO: Replace with your actual GCS bucket name
    prefix  = "the-academy-sync/state"
  }
}
```

Replace `"your-gcs-bucket-name-here"` with the actual name of the GCS bucket you created.

### Initializing Terraform

After creating the GCS bucket and updating `backend.tf`, navigate to the `terraform` directory in your terminal and run the following command to initialize Terraform:

```sh
terraform init
```

This command will download the necessary provider plugins and configure the backend to use your GCS bucket. You should see a message like "Successfully configured the backend 'gcs'".

After successful initialization, any `terraform apply` commands will store the state in the configured GCS bucket.
