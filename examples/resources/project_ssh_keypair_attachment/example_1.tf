variable "deploy_public_key" {
  type = string
}

variable "deploy_private_key" {
  type      = string
  sensitive = true
}

resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_ssh_keypair" "deploy_key" {
  handle      = "deploy-key"
  name        = "Deploy key"
  public_key  = var.deploy_public_key
  private_key = var.deploy_private_key
}

resource "bahriya_project_ssh_keypair_attachment" "web_ssh_keypair" {
  project_id = bahriya_project.web.id
  handle     = bahriya_ssh_keypair.deploy_key.handle
}
