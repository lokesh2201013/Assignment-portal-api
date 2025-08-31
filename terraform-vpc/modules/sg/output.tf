output "sg_id" {
  value = google_compute_firewall.sg.self_link
}
output "instance_tag" {
  value = ["allow-http-ssh"]  # Define the tag to be used
}