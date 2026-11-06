data "bahriya_regions" "active" {
  status_filter = "active"
}

resource "bahriya_project" "global" {
  handle  = "global-app"
  name    = "Global Application"
  regions = [for r in data.bahriya_regions.active.regions : r.id]
}
