resource "google_compute_network" "my_vpc" {
 name = "my-vpc"
 auto_create_subnetworks = false
 description = "Custom vpc network"
 
}

resource "google_compute_subnetwork" "subnets" {
  count = length(var.subnet_cidr)
  name = var.subnet_names[count.index]
  network = google_compute_network.my_vpc.self_link
  ip_cidr_range = var.subnet_cidr[count.index]
}

resource "google_compute_firewall" "allow_all" {
  name    = "allow-all"
  network = google_compute_network.my_vpc.self_link

  allow {
    protocol = "all"
  }

  source_ranges = ["0.0.0.0/0"]
}
/*resource "google_compute_route" "default_route" {
  name         = "default-route"
  network      = google_compute_network.my_vpc.self_link
  dest_range   = "0.0.0.0/0"
  next_hop_gateway = "https://www.googleapis.com/compute/v1/projects/${var.project_id}/global/gateways/default-internet"
  priority     = 1000
}*/
