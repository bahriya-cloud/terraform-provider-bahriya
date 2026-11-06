# Terminates TLS itself — probe over HTTPS. The certificate is not verified,
# so a self-signed one is fine.
resource "bahriya_container" "secure_api" {
  handle            = "secure-api"
  name              = "Secure API"
  image             = "ghcr.io/org/secure-api:2.0.0"
  containerport     = "8443"
  healthcheckpath   = "/healthz"
  protocol          = "https"
  mincpu            = "100"
  minmemory         = "128"
  activeregions     = ["falkenstein-1"]
  project           = bahriya_project.web.id
}

# Speaks a binary protocol, not HTTP — probe the port only.
resource "bahriya_container" "queue_bridge" {
  handle            = "queue-bridge"
  name              = "Queue Bridge"
  image             = "registry.example.com/team/bridge:1.4.2"
  containerport     = "9000"
  protocol          = "tcp"
  mincpu            = "50"
  minmemory         = "128"
  activeregions     = ["falkenstein-1"]
  project           = bahriya_project.web.id
}
