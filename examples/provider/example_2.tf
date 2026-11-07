terraform {
  required_providers {
    bahriya = {
      source  = "bahriya-cloud/bahriya"
      version = "~> 1.0"
    }
  }
}

# The token and organisation come from BAHRIYA_TOKEN and BAHRIYA_ORGANISATION_ID.
provider "bahriya" {}

data "bahriya_regions" "active" {
  status_filter = "active"
}

resource "bahriya_project" "web" {
  handle  = "web-prod"
  name    = "Web Production"
  regions = [data.bahriya_regions.active.regions[0].id]
}

resource "bahriya_container" "api" {
  handle  = "api"
  name    = "API Server"
  project = bahriya_project.web.id
  regions = [data.bahriya_regions.active.regions[0].id]

  workload = {
    image       = "ghcr.io/myorg/api:1.2.3"
    port        = "3000"
    healthcheck = "/healthz"
    cpu         = "250"
    memory      = "256"
  }

  scaling = {
    replicas = "2"
  }
}
