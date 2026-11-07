variable "signing_public_key_path" {
  type        = string
  description = "Path to the ASCII-armoured public key, such as signing.pub.asc."
}

variable "signing_private_key" {
  type        = string
  sensitive   = true
  description = "The ASCII-armoured private key."
}

resource "bahriya_gpg_keypair" "signing" {
  handle      = "release-signing"
  name        = "Release Signing Key"
  public_key  = file(var.signing_public_key_path)
  private_key = var.signing_private_key
}
