terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.42.0"
    }
  }

  backend "gcs"{
    bucket = "backend-tf-cicd-gke-2"
    prefix = "terraform/state"
  }
}

provider "google" {
  project     = "digital-vim-454610-e0"
  region      = "us-east1"
  
}