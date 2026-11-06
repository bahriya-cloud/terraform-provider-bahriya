resource "bahriya_env_file" "app_env" {
  # ... bahriya_env_file configuration ...
}

resource "bahriya_project_env_file_attachment" "web_env_file" {
  project_id = bahriya_project.web.id
  handle     = bahriya_env_file.app_env.handle
}
