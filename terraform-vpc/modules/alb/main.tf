resource "google_compute_instance_group" "default" {
  name = "instance-group"
  zone = "us-east1-b"
  instances =[
    var.instancegrp[0],
    var.instancegrp[1]
  ]
}


resource "google_compute_http_health_check" "http_health_check" {
  name               = "http-health-check"
  request_path        = "/"
  port               = 80
  check_interval_sec = 30
  timeout_sec        = 10
  healthy_threshold  = 2
  unhealthy_threshold = 2
}


resource "google_compute_backend_service" "alb" {
name = "application-load-balancer"
protocol = "HTTP"
backend {
  group = google_compute_instance_group.default.self_link
}
port_name = "http"
 health_checks = [
    google_compute_http_health_check.http_health_check.self_link
  ]
}

resource "google_compute_url_map" "http_url_map" {
  name = "http-url-map"
  default_service = google_compute_backend_service.alb.self_link
}

resource "google_compute_target_http_proxy" "http_proxy" {
  name="http-target-proxy"
  url_map=google_compute_url_map.http_url_map.self_link
}

resource "google_compute_global_forwarding_rule" "http_forwarding_rule" {
  name = "http-forwarding-rule"
  target = google_compute_target_http_proxy.http_proxy.self_link
  port_range = "80"
  ip_protocol = "TCP"
}