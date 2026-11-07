resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_json_config" "feature_flags" {
  handle  = "feature-flags"
  name    = "Feature flags"
  content = jsonencode({ new_checkout = true })
}

resource "bahriya_project_json_config_attachment" "web_json_config" {
  project_id = bahriya_project.web.id
  handle     = bahriya_json_config.feature_flags.handle
}
