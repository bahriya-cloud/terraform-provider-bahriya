resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_plain_config" "nginx_conf" {
  handle  = "nginx-conf"
  name    = "Nginx configuration"
  content = "worker_processes auto;\n"
}

resource "bahriya_project_plain_config_attachment" "web_plain_config" {
  project_id = bahriya_project.web.id
  handle     = bahriya_plain_config.nginx_conf.handle
}
