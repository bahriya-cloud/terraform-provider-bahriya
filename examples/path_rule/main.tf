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

# The parent HTTP container — replace with your own.
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
  image   = "ghcr.io/myorg/api:1.0.0"
  project = bahriya_project.app.id

  cpu             = "500"
  memory          = "512"
  containerport   = "8080"
  healthcheckpath = "/health"
  activeregions   = [data.bahriya_regions.active.regions[0].id]
}

# Lock down /api/admin behind basic authentication.
resource "bahriya_path_rule" "admin" {
  container_id = bahriya_container.api.id
  handle       = "admin-area"
  path         = "/api/admin"
  priority     = 100

  basicauthenabled = true
  basicauthcredentials = [
    {
      username = "alice"
      password = var.admin_password
    },
  ]

  ipwhitelistenabled = true
  ipwhitelist        = ["10.0.0.0/8"]
}

# Rate-limit /webhook independently of the container-wide settings.
resource "bahriya_path_rule" "webhook" {
  container_id = bahriya_container.api.id
  handle       = "webhook"
  path         = "/webhook"

  ratelimitingenabled           = true
  ratelimitingrequestsperminute = 60
  ratelimitingrequestsperhour   = 1000
}
