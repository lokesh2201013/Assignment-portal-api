env = "dev"
location = "us-east1"
region = "us-east1"
vpc_name = "vpc"
zone = "us-east1-b"

pub_sub_name = "public-subnet"
pub_subnet_count = 2
pub_cidr_block = [ "10.16.0.0/20","10.16.16.0/20" ]
pub_availability_zone = [ "us-east1-a", "us-east1-b" ]
public_rt_name = "public-route-table"

ngw_name = "ngw"
gke_cluster_sg = "gke-sg"

is_gke_cluster_enabled = true
cluster_version = ""
cluster_name = "gke-cluster"

master_password = "password"
is_ondemand_node_pool_enabled = true
ondemand_instance_type = "e2-medium"
desired_capacity_on_demand = 3
min_capacity_on_demand = 1
max_capacity_on_demand = 3

addons = [ {
  name = "kubernetes-dashboard"
  enabled = true
} ,  {
    name    = "cloud-monitoring"
    version = "1.0.0"
  },
  {
    name    = "cloud-logging"
    version = "1.0.0"
  },]