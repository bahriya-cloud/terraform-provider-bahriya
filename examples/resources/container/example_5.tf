resource "bahriya_project" "web" {
  name    = "Web"
  handle  = "web"
  regions = ["falkenstein-1"]
}

resource "bahriya_container" "api" {
  name    = "Rate Limited API"
  handle  = "rate-limited-api"
  project = bahriya_project.web.id
  regions = ["falkenstein-1"]

  workload = {
    image       = "nginx:alpine"
    port        = 80
    healthcheck = "/"
    cpu         = 100
    memory      = 128
  }

  scaling = {
    replicas = 1
  }

  networking = {
    external = true
  }

  security = {
    rate_limiting = {
      enabled    = true
      per_second = 10
      per_minute = 300
    }

    ip_allowlist = {
      enabled = true
      ips     = ["203.0.113.0/24", "198.51.100.0/24"]
    }
  }
}
