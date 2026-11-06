data "bahriya_regions" "active" {
  status_filter = "active"
}

resource "bahriya_project" "web" {
  handle  = "web-prod"
  name    = "Web Production"
  regions = [data.bahriya_regions.active.regions[0].id]
}
