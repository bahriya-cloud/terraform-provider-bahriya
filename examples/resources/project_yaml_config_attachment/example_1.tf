resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_yaml_config" "app_config" {
  handle  = "app-config"
  name    = "Application configuration"
  content = yamlencode({ log_level = "info" })
}

resource "bahriya_project_yaml_config_attachment" "web_yaml_config" {
  project_id = bahriya_project.web.id
  handle     = bahriya_yaml_config.app_config.handle
}
