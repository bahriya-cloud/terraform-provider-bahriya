terraform {
  required_providers {
    bahriya = {
      source = "registry.terraform.io/bahriya-cloud/bahriya"
    }
  }
}

provider "bahriya" {}

variable "admin_password" {
  description = "Password for the basic auth credential on /api/admin"
  type        = string
  sensitive   = true
}

# The parent HTTP container. Replace it with your own.
data "bahriya_regions" "active" {
  status_filter = "active"
}

resource "bahriya_project" "app" {
  handle  = "path-rules-demo"
  name    = "Path Rules Demo"
  regions = [data.bahriya_regions.active.regions[0].id]
}

resource "bahriya_container" "api" {
  handle  = "my-api"
  name    = "Public API"
  type    = "http"
  project = bahriya_project.app.id
  regions = [data.bahriya_regions.active.regions[0].id]

  workload = {
    image       = "ghcr.io/myorg/api:1.0.0"
    port        = 8080
    healthcheck = "/health"
    cpu         = 500
    memory      = 512
  }

  networking = {
    external = true
  }
}

# Protect /api/admin with basic authentication and an IP allowlist.
resource "bahriya_path_rule" "admin" {
  container = bahriya_container.api.id
  handle    = "admin-area"
  path      = "/api/admin"
  priority  = 100

  security = {
    basic_auth_enabled = true
    basic_auth = [
      {
        username = "alice"
        password = var.admin_password
      },
    ]

    ip_allowlist = {
      enabled = true
      ips     = ["10.0.0.0/8"]
    }
  }
}

# Rate-limit /webhook independently of the container-wide settings.
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
