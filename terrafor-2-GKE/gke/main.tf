locals {
  env= var.env
}


module "gke" {
    region = var.region
    location= var.location
    source="../modules"
    env= var.env
    clsuter_name= "${local.env}-${var.cluster_name}"
}