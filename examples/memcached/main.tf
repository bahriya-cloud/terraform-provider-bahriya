terraform {
  required_providers {
    bahriya = {
      source = "registry.terraform.io/bahriya-cloud/bahriya"
    }
  }
}

provider "bahriya" {}

data "bahriya_regions" "active" {
  status_filter = "active"
}

resource "bahriya_project" "cache" {
  handle  = "cache-project"
  name    = "Cache Infrastructure"
  regions = [data.bahriya_regions.active.regions[0].id]
}

resource "bahriya_memcached" "session" {
  handle        = "session-cache"
  name          = "Session Cache"
  memorymb      = 512
  activeregions = [data.bahriya_regions.active.regions[0].id]
  project       = bahriya_project.cache.id
}

output "memcached_id" {
  value = bahriya_memcached.session.id
}

output "memcached_status" {
  value = bahriya_memcached.session.status
}
