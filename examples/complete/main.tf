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
  handle        = "app-cache"
  name          = "Application Cache"
  memorymb      = 512
  activeregions = [local.primary_region]
  project       = bahriya_project.main.id
}

# ---------- Container ----------

resource "bahriya_container" "api" {
  handle = "api-server"
  name   = "API Server"
  image  = "myorg/api:latest"

  # Required for HTTP containers
  containerport   = "3000"
  healthcheckpath = "/healthz"

  mincpu    = "500"
  minmemory = "512"

  autoscalingminreplicas = "2"
  autoscalingmaxreplicas = "5"

  activeregions = [local.primary_region]

  registry = bahriya_registry.docker_hub.handle
  project  = bahriya_project.main.id

  newenvvar {
    key   = "NODE_ENV"
    value = "production"
  }

  newenvvar {
    key   = "CACHE_HOST"
    value = "app-cache.memcached.bahriya.internal"
  }

  secretsenvvar {
    secret = bahriya_secret.db_url.handle
    name   = "DATABASE_URL"
  }

  hostnames {
    hostname = "api.example.com"
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
