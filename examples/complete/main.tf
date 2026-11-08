terraform {
  required_providers {
    bahriya = {
      source = "registry.terraform.io/bahriya-cloud/bahriya"
    }
  }
}

provider "bahriya" {}

# ---------- Data Sources ----------

data "bahriya_organisation" "current" {}

data "bahriya_regions" "active" {
  status_filter = "active"
}

locals {
  primary_region = data.bahriya_regions.active.regions[0].id
}

# ---------- Project ----------

resource "bahriya_project" "main" {
  handle  = "complete-example"
  name    = "Complete Example"
  regions = [local.primary_region]
}

# ---------- Registry ----------

resource "bahriya_registry" "docker_hub" {
  handle   = "docker-hub"
  name     = "Docker Hub"
  server   = "registry-1.docker.io"
  username = var.docker_username
  password = var.docker_password
}

# ---------- Secrets ----------

resource "bahriya_secret" "db_url" {
  handle = "db-url"
  name   = "Database URL"
  value  = var.database_url
}

# ---------- Cache ----------

resource "bahriya_memcached" "cache" {
  handle  = "app-cache"
  name    = "Application Cache"
  regions = [local.primary_region]
  project = bahriya_project.main.id

  tuning = {
    memory = 512
  }
}

# ---------- Container ----------

resource "bahriya_container" "api" {
  handle  = "api-server"
  name    = "API Server"
  project = bahriya_project.main.id
  regions = [local.primary_region]

  workload = {
    image    = "myorg/api:latest"
    registry = bahriya_registry.docker_hub.id

    # An HTTP container needs a port and a health check path.
    port        = 3000
    healthcheck = "/healthz"

    cpu    = 500
    memory = 512

    env = {
      "NODE_ENV"   = "production"
      "CACHE_HOST" = "app-cache.memcached.bahriya.internal"
    }

    # Environment variable name = secret handle.
    secrets = {
      "DATABASE_URL" = bahriya_secret.db_url.handle
    }
  }

  scaling = {
    replicas     = 2
    max_replicas = 5
  }

  networking = {
    # Required for an HTTP container: true serves it at its hostnames.
    external = true
    domains = [
      { hostname = "api.example.com" },
    ]
  }
}

# ---------- Variables ----------

variable "docker_username" {
  type = string
}

variable "docker_password" {
  type      = string
  sensitive = true
}

variable "database_url" {
  type      = string
  sensitive = true
}

# ---------- Outputs ----------

output "organisation" {
  value = {
    name   = data.bahriya_organisation.current.name
    handle = data.bahriya_organisation.current.handle
  }
}

output "regions" {
  value = [for r in data.bahriya_regions.active.regions : r.id]
}

output "project_id" {
  value = bahriya_project.main.id
}

output "container_status" {
  value = bahriya_container.api.status
}

output "cache_status" {
  value = bahriya_memcached.cache.status
}
