terraform {
  required_providers {
    bahriya = {
      source = "registry.terraform.io/bahriya-cloud/bahriya"
    }
  }
}

provider "bahriya" {}

# Use a data source to dynamically select regions
data "bahriya_regions" "active" {
  status_filter = "active"
}

# Project to hold all resources
resource "bahriya_project" "app" {
  handle  = "my-app"
  name    = "My Application"
  regions = [data.bahriya_regions.active.regions[0].id]
}

# Private registry credentials
resource "bahriya_registry" "ghcr" {
  handle   = "ghcr-registry"
  name     = "GitHub Container Registry"
  server   = "ghcr.io"
  username = var.ghcr_username
  password = var.ghcr_token
}

# Application secrets
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

# Container deployment
resource "bahriya_container" "web" {
  handle = "web-app"
  name   = "Web Application"
  image  = "ghcr.io/myorg/web-app:latest"

  # Required for HTTP containers
  containerport   = "8080"
  healthcheckpath = "/healthz"

  mincpu    = "250"
  minmemory = "256"

  autoscalingminreplicas = "1"
  autoscalingmaxreplicas = "3"

  activeregions = [data.bahriya_regions.active.regions[0].id]

  registry = bahriya_registry.ghcr.handle
  project  = bahriya_project.app.id

  newenvvar {
    key   = "NODE_ENV"
    value = "production"
  }

  secretsenvvar {
    secret = bahriya_secret.db_url.handle
    name   = "DATABASE_URL"
  }

  secretsenvvar {
    secret = bahriya_secret.api_key.handle
    name   = "API_KEY"
  }

  hostnames {
    hostname = "app.example.com"
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
