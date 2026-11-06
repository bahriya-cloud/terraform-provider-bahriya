resource "bahriya_json_config" "feature_flags" {
  # ... bahriya_json_config configuration ...
}

resource "bahriya_project_json_config_attachment" "web_json_config" {
  project_id = bahriya_project.web.id
  handle     = bahriya_json_config.feature_flags.handle
}
