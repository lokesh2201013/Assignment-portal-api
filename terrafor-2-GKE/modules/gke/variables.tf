variable "cluster_name" {
  description = "The name of the Kubernetes cluster."
}

variable "vpc_name" {
  description = "The name of the VPC."
}

variable "pub_subnet_count" {
  description = "The number of public subnets to create."
}

variable "pub_sub_name" {
  description = "The base name for public subnets."
}

variable "pub_cidr_block" {
  description = "A list of CIDR blocks for the public subnets."
}

variable "pub_availability_zone" {
  description = "A list of availability zones for the public subnets."
}


variable "region" {
  description = "The region where resources will be created."
}

variable "zone" {
  description = "The zone where resources will be created."
  
}

variable "ngw_name" {
  description = "The name of the NAT Gateway."
}

variable "public_rt_name" {
  description = "The name of the public route."
}

variable "gke_cluster_sg" {
  description = "The name of the GKE security group."
}

variable "env" {
  description = "The environment tag (e.g., 'dev', 'prod')."
}

variable "is_gke_cluster_enabled" {
  description = "Boolean flag to enable or disable the creation of a Google Kubernetes Engine (GKE) cluster."
}

variable "location" {
  description = "The location (region) where the GKE cluster and other resources will be deployed."
}

variable "cluster_version" {
  description = "The Kubernetes version to be used for the GKE cluster."
}

variable "master_password" {
  description = "The password for the master user of the GKE cluster."
  sensitive   = true
}

variable "is_ondemand_node_pool_enabled" {
  description = "Boolean flag to enable or disable the on-demand node pool in the GKE cluster."
}

variable "ondemand_instance_type" {
  description = "The machine type to be used for the on-demand node pool."
}

variable "desired_capacity_on_demand" {
  description = "The desired number of nodes for the on-demand node pool."
}

variable "min_capacity_on_demand" {
  description = "The minimum number of nodes for the on-demand node pool."
}

variable "max_capacity_on_demand" {
  description = "The maximum number of nodes for the on-demand node pool."
}

variable "addons" {
  description = "Additional add-ons to be deployed in the cluster."
}

