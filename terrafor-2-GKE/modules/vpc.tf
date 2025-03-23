locals{
    cluster_name = var.cluster_name
    env = var.env
}

resource "google_compute_network" "vpc" {
  name= var.vpc_name
  description = "VPC for ${local.cluster_name} ${local.env}"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "public_subnet" {
  count = var.pub_subnet_count
  name="${var.pub_sub_name}-${count.index}"
}