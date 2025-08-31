terraform {
  backend "gcs" {
    bucket  = <ADD your OWN>
    prefix  = <ADD YOUR OWN>
  }
}

provider "google" {
  project = <ADD YOUR OWN>
  region  = "us-east1"
}
