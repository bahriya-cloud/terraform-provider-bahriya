variable "ghcr_username" {
  type = string
}

variable "ghcr_token" {
  type      = string
  sensitive = true
}

resource "bahriya_registry" "ghcr" {
  handle   = "ghcr"
  name     = "GitHub Container Registry"
  server   = "ghcr.io"
  username = var.ghcr_username
  password = var.ghcr_token
}
