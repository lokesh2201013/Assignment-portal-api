resource "google_compute_firewall" "sg" {
  name    = "allow-http-ssh"
  network = "default"
  project = var.project_id
  allow {
    protocol = "tcp"
    ports    = ["80", "22"]
  }

  source_ranges = ["0.0.0.0/0"]
}