resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_env_file" "app_env" {
  handle  = "app-env"
  name    = "Application environment"
  content = "LOG_LEVEL=info\nCACHE_TTL=300\n"
}

resource "bahriya_project_env_file_attachment" "web_env_file" {
  project_id = bahriya_project.web.id
  handle     = bahriya_env_file.app_env.handle
}
