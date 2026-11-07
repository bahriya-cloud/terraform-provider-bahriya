variable "signing_public_key" {
  type = string
}

variable "signing_private_key" {
  type      = string
  sensitive = true
}

resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_gpg_keypair" "signing_key" {
  handle      = "signing-key"
  name        = "Signing key"
  public_key  = var.signing_public_key
  private_key = var.signing_private_key
}

resource "bahriya_project_gpg_keypair_attachment" "web_gpg_keypair" {
  project_id = bahriya_project.web.id
  handle     = bahriya_gpg_keypair.signing_key.handle
}
