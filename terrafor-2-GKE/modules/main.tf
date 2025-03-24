locals {
  env = var.env
}

module "gke" {
  source       = "./modules/gke"
  location     = var.location
  env          = local.env
  cluster_name = "${local.env}-${var.cluster_name}"
  vpc_name     = var.vpc_name

  pub_subnet_count = var.pub_subnet_count
  pub_sub_name = var.pub_sub_name
  pub_cidr_block = var.pub_cidr_block
  pub_availability_zone = var.pub_availability_zone
  public_rt_name = var.public_rt_name
  ngw_name = var.ngw_name

  gke_cluster_sg = var.gke_cluster_sg
  ondemand_instance_type = var.ondemand_instance_type
  desired_capacity_on_demand = var.desired_capacity_on_demand
  min_capacity_on_demand = var.min_capacity_on_demand
  max_capacity_on_demand = var.max_capacity_on_demand
  is_gke_cluster_enabled = var.is_gke_cluster_enabled
  cluster_version = var.cluster_version

  is_ondemand_node_pool_enabled = var.is_ondemand_node_pool_enabled
  addons = var.addons
}

