variable "contractor_user_id" {
  type = string
}

variable "ghcr_username" {
  type = string
}

variable "ghcr_token" {
  type      = string
  sensitive = true
}

resource "bahriya_registry" "ghcr" {
  name     = "GitHub Container Registry"
  handle   = "ghcr"
  server   = "ghcr.io"
  username = var.ghcr_username
  password = var.ghcr_token
}

resource "bahriya_resource_grant" "share_registry" {
  to_user       = var.contractor_user_id
  resource_type = "attachables_registries"
  resource_id   = bahriya_registry.ghcr.id
  permissions   = ["read"]
}
