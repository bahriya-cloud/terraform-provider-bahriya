resource "bahriya_project" "web" {
  name    = "Web"
  handle  = "web"
  regions = ["falkenstein-1"]
}

output "cpu_headroom" {
  value = bahriya_project.web.effective_quotas.request.cpu
}
