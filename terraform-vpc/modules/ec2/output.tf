output "instance" {
  value = google_compute_instance.web_instances[*].self_link
}


output "subnets" {
  value =var.subnets
}

output "ec2_names" {
  value = ["w1","w2"]
}