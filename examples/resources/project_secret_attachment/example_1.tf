resource "bahriya_secret" "db_password" {
  # ... bahriya_secret configuration ...
}

resource "bahriya_project_secret_attachment" "web_secret" {
  project_id = bahriya_project.web.id
  handle     = bahriya_secret.db_password.handle
}
