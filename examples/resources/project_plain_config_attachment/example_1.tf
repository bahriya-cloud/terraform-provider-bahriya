resource "bahriya_plain_config" "nginx_conf" {
  # ... bahriya_plain_config configuration ...
}

resource "bahriya_project_plain_config_attachment" "web_plain_config" {
  project_id = bahriya_project.web.id
  handle     = bahriya_plain_config.nginx_conf.handle
}
