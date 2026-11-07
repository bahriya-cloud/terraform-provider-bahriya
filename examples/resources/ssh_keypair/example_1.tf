variable "deploy_public_key_path" {
  type        = string
  description = "Path to the public key line, such as deploy.pub."
}

variable "deploy_private_key" {
  type        = string
  sensitive   = true
  description = "The private key, PEM-encoded."
}

resource "bahriya_ssh_keypair" "deploy" {
  handle      = "deploy"
  name        = "Deploy Key"
  public_key  = file(var.deploy_public_key_path)
  private_key = var.deploy_private_key
}
