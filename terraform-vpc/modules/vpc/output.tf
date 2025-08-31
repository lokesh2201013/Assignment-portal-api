output "network_id" {
  value = google_compute_network.my_vpc.self_link
}

output "subnets_id" {
  value = google_compute_subnetwork.subnets.*.self_link
}

