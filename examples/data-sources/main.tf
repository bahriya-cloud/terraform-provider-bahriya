terraform {
  required_providers {
    bahriya = {
      source = "registry.terraform.io/bahriya-cloud/bahriya"
    }
  }
}

provider "bahriya" {}

# Fetch the current organisation
data "bahriya_organisation" "current" {}

output "org_name" {
  value = data.bahriya_organisation.current.name
}

output "org_handle" {
  value = data.bahriya_organisation.current.handle
}

# Fetch all active regions
data "bahriya_regions" "active" {
  status_filter = "active"
}

output "active_regions" {
  value = data.bahriya_regions.active.regions[*].id
}

# Fetch a specific region by ID
data "bahriya_region" "helsinki" {
  id = "helsinki-1"
}

output "helsinki_name" {
  value = data.bahriya_region.helsinki.name
}

output "helsinki_country" {
  value = data.bahriya_region.helsinki.country
}
