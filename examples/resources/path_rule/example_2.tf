resource "bahriya_project" "web" {
  name    = "Web"
  handle  = "web"
  regions = ["falkenstein-1"]
}

resource "bahriya_container" "api" {
  name    = "API"
  handle  = "api"
  project = bahriya_project.web.id
  regions = ["falkenstein-1"]

  workload = {
    image  = "ghcr.io/myorg/api:1.2.3"
    port   = 3000
    cpu    = 250
    memory = 256
  }

  scaling = {
    replicas = 1
  }

  networking = {
    external = true
  }
}

resource "bahriya_path_rule" "webhook" {
  container = bahriya_container.api.id
  handle    = "webhook"
  path      = "/webhook"

  security = {
    rate_limiting = {
      enabled    = true
      per_minute = 60
      per_hour   = 1000
    }
  }
}
