variable "ghcr_username" {
  type = string
}

variable "ghcr_token" {
  type      = string
  sensitive = true
}

resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_registry" "ghcr" {
  handle   = "ghcr"
  name     = "GitHub Container Registry"
  server   = "ghcr.io"
  username = var.ghcr_username
  password = var.ghcr_token
}

resource "bahriya_project_registry_attachment" "web_registry" {
  project_id = bahriya_project.web.id
  handle     = bahriya_registry.ghcr.handle
}
