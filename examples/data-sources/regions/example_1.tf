data "bahriya_regions" "active" {
  status_filter = "active"
}

output "region_ids" {
  value = [for r in data.bahriya_regions.active.regions : r.id]
}
