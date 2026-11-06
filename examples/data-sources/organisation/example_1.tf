data "bahriya_organisation" "current" {}

output "org_name" {
  value = data.bahriya_organisation.current.name
}

output "org_handle" {
  value = data.bahriya_organisation.current.handle
}
