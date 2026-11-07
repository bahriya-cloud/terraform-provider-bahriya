resource "bahriya_project" "app" {
  handle  = "app"
  name    = "App"
  regions = ["falkenstein-1"]
}

data "bahriya_project_quota" "app" {
  project = bahriya_project.app.id
}

locals {
  # Every region carries the same ceiling, so the first entry is representative.
  ceiling = data.bahriya_project_quota.app.regions[0].available.cpu_ceiling
}

output "project_cpu_ceiling" {
  value = local.ceiling
}
