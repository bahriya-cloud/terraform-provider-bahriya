variable "db_password" {
  type      = string
  sensitive = true
}

resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_secret" "db_password" {
  handle = "db-password"
  name   = "Database password"
  value  = var.db_password
}

resource "bahriya_project_secret_attachment" "web_secret" {
  project_id = bahriya_project.web.id
  handle     = bahriya_secret.db_password.handle
}
