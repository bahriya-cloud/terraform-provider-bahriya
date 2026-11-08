# A container pulls its image from a registry, referenced by the registry's ID.

variable "ghcr_username" {
  type = string
}

variable "ghcr_token" {
  type      = string
  sensitive = true
}

resource "bahriya_registry" "ghcr" {
  handle   = "ghcr"
  name     = "GitHub Container Registry"
  server   = "ghcr.io"
  username = var.ghcr_username
  password = var.ghcr_token
}

resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_container" "api" {
  project = bahriya_project.web.id
  handle  = "api"
  name    = "API Server"
  regions = ["falkenstein-1"]

  workload = {
    image    = "ghcr.io/myorg/api:1.0.0"
    registry = bahriya_registry.ghcr.id
    cpu      = 250
    memory   = 256
  }

  networking = {
    external = true
  }
}
