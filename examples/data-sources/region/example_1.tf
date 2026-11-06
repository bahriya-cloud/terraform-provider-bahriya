data "bahriya_region" "helsinki" {
  id = "helsinki-1"
}

output "region_name" {
  value = data.bahriya_region.helsinki.name
}

output "region_country" {
  value = data.bahriya_region.helsinki.country
}
