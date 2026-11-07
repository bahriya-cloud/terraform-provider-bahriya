resource "bahriya_project" "web" {
  name    = "Web"
  handle  = "web"
  regions = ["falkenstein-1"]
}

# Terminates TLS itself, so the health check probes over HTTPS. The
# certificate is not verified, so a self-signed certificate is acceptable.
resource "bahriya_container" "secure_api" {
  name    = "Secure API"
  handle  = "secure-api"
  project = bahriya_project.web.id
  regions = ["falkenstein-1"]

  workload = {
    image       = "ghcr.io/org/secure-api:2.0.0"
    port        = "8443"
    healthcheck = "/healthz"
    protocol    = "https"
    cpu         = "100"
    memory      = "128"
  }

  scaling = {
    replicas = "1"
  }
}

# Speaks a binary protocol rather than HTTP, so only the port is probed.
resource "bahriya_container" "queue_bridge" {
  name    = "Queue Bridge"
  handle  = "queue-bridge"
  project = bahriya_project.web.id
  regions = ["falkenstein-1"]

  workload = {
    image    = "registry.example.com/team/bridge:1.4.2"
    port     = "9000"
    protocol = "tcp"
    cpu      = "50"
    memory   = "128"
  }

  scaling = {
    replicas = "1"
  }
}
