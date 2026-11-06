resource "bahriya_yaml_config" "app_config" {
  # ... bahriya_yaml_config configuration ...
}

resource "bahriya_project_yaml_config_attachment" "web_yaml_config" {
  project_id = bahriya_project.web.id
  handle     = bahriya_yaml_config.app_config.handle
}
