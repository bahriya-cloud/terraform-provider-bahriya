variable "session_token_key" {
  type      = string
  sensitive = true
}

resource "bahriya_encryption_key" "tokens" {
  handle = "session-tokens"
  name   = "Session token AES key"
  format = "base64"
  key    = var.session_token_key
}
