variable "ops_password" {
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

resource "bahriya_path_rule" "internal" {
  container = bahriya_container.api.id
  handle    = "internal-tools"
  path      = "/internal"
  priority  = 100

  security = {
    ip_allowlist = {
      enabled = true
      ips     = ["10.0.0.0/8", "192.168.0.0/16"]
    }

    basic_auth_enabled = true
    basic_auth = [
      {
        username = "ops"
        password = var.ops_password
      },
    ]
  }
}
