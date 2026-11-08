variable "admin_password" {
  type      = string
  sensitive = true
}

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

resource "bahriya_path_rule" "admin" {
  container = bahriya_container.api.id
  handle    = "admin"
  path      = "/api/admin"

  security = {
    basic_auth_enabled = true
    basic_auth = [
      {
        username = "alice"
        password = var.admin_password
      },
    ]
  }
}
