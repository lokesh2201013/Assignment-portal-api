resource "google_container_cluster" "gke" {
  count               = var.is_gke_cluster_enabled ? 1 : 0
  name               = var.cluster_name
  location           = var.zone
  initial_node_count = 1
  min_master_version = var.cluster_version
  network           = google_compute_network.vpc.self_link
  subnetwork        = google_compute_subnetwork.public_subnet[0].self_link  # ✅ Fix here
  deletion_protection = false

  master_auth {
    client_certificate_config {
      issue_client_certificate = false
    }
  }

}


resource "google_container_node_pool" "ondemand" {
  count       = var.is_ondemand_node_pool_enabled ? 1 : 0
  name        = "${var.cluster_name}-ondemand"
  location    = var.zone
  cluster     = google_container_cluster.gke[0].name
  initial_node_count = var.min_capacity_on_demand  # ✅ Keep this
  
  node_config {
    machine_type = var.ondemand_instance_type
    disk_size_gb = 10
  }

  autoscaling {
    min_node_count = var.min_capacity_on_demand
    max_node_count = var.max_capacity_on_demand
  }

  management {
    auto_repair  = true
    auto_upgrade = true
  }
}
