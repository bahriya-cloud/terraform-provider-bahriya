terraform {
  required_providers {
    bahriya = {
      source = "registry.terraform.io/bahriya-cloud/bahriya"
    }
  }
}

provider "bahriya" {}

# ---------- Project ----------

resource "bahriya_project" "e2e" {
  handle  = "tf-e2e-prod"
  name    = "TF E2E Production Test"
  regions = ["helsinki-1", "falkenstein-1"]
}

# ---------- Memcached ----------

resource "bahriya_memcached" "e2e" {
  handle        = "tf-e2e-cache"
  name          = "TF E2E Cache"
  memorymb      = 512
  activeregions = ["helsinki-1"]
  project       = bahriya_project.e2e.id
}

# ---------- Container ----------

resource "bahriya_container" "e2e" {
  handle    = "tf-e2e-web3"
  name      = "TF E2E Web3"
  image     = "nginx:alpine"
  mincpu    = "100"
  minmemory = "128"

  containerport   = "80"
  healthcheckpath = "/"

  autoscalingminreplicas = "1"

  activeregions = ["falkenstein-1"]
  project       = bahriya_project.e2e.id
}

# ---------- Outputs ----------

output "project_id" {
  value = bahriya_project.e2e.id
}

output "memcached_id" {
  value = bahriya_memcached.e2e.id
}

output "memcached_status" {
  value = bahriya_memcached.e2e.status
}

output "container_id" {
  value = bahriya_container.e2e.id
}

output "container_status" {
  value = bahriya_container.e2e.status
}
