resource "bahriya_env_file" "api" {
  handle  = "api-env"
  name    = "API env"
  content = <<-EOT
    # API runtime configuration
    LOG_LEVEL=info
    METRICS_ENABLED=true
    REGION=eu-west
  EOT
}
