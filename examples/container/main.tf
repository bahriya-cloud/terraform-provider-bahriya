terraform {
  required_providers {
    bahriya = {
      source = "registry.terraform.io/bahriya-cloud/bahriya"
    }
  }
}

provider "bahriya" {}

# A data source selects the regions dynamically.
data "bahriya_regions" "active" {
  status_filter = "active"
}

# The project that holds all resources.
resource "bahriya_project" "app" {
  handle  = "my-app"
  name    = "My Application"
  regions = [data.bahriya_regions.active.regions[0].id]
}

# Credentials for a private registry.
resource "bahriya_registry" "ghcr" {
  handle   = "ghcr-registry"
  name     = "GitHub Container Registry"
  server   = "ghcr.io"
  username = var.ghcr_username
  password = var.ghcr_token
}

# Application secrets.
resource "bahriya_secret" "db_url" {
  handle = "database-url"
  name   = "Database URL"
  value  = var.database_url
}

resource "bahriya_secret" "api_key" {
  handle = "api-key"
  name   = "API Key"
  value  = var.api_key
}

# The container deployment.
resource "bahriya_container" "web" {
  handle  = "web-app"
  name    = "Web Application"
  project = bahriya_project.app.id
  regions = [data.bahriya_regions.active.regions[0].id]

  workload = {
    image    = "ghcr.io/myorg/web-app:latest"
    registry = bahriya_registry.ghcr.id

    # An HTTP container needs a port and a health check path.
    port        = 8080
    healthcheck = "/healthz"

    cpu    = 250
    memory = 256

    env = {
      "NODE_ENV" = "production"
    }

    # Environment variable name = secret handle.
    secrets = {
      "DATABASE_URL" = bahriya_secret.db_url.handle
      "API_KEY"      = bahriya_secret.api_key.handle
    }
  }

  scaling = {
    replicas     = 1
    max_replicas = 3
  }

  networking = {
    # Required for an HTTP container: true serves it at its hostnames.
    external = true
    domains = [
      { hostname = "app.example.com" },
    ]
  }
}

variable "ghcr_username" {
  type = string
}

variable "ghcr_token" {
  type      = string
  sensitive = true
}

variable "database_url" {
  type      = string
  sensitive = true
}

variable "api_key" {
  type      = string
  sensitive = true
}

output "container_id" {
  value = bahriya_container.web.id
}

output "container_status" {
  value = bahriya_container.web.status
}
