resource "bahriya_yaml_config" "app" {
  handle  = "app-yaml"
  name    = "App YAML"
  content = <<-EOT
    server:
      port: 8080
      timeout: 30s
    features:
      beta: false
  EOT
}
