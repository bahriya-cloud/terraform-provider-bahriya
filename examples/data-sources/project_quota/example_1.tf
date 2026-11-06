data "bahriya_project_quota" "app" {
  project = bahriya_project.app.id
}

output "falkenstein_cpu_headroom" {
  value = one([
    for r in data.bahriya_project_quota.app.regions : r.available.cpu_ceiling
    if r.region == "falkenstein-1"
  ])
}
