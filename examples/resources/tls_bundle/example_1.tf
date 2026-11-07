variable "ca_path" {
  type        = string
  description = "Path to the PEM-encoded CA certificate."
}

variable "cert_path" {
  type        = string
  description = "Path to the PEM-encoded server certificate."
}

variable "tls_private_key" {
  type        = string
  sensitive   = true
  description = "The PEM-encoded private key."
}

resource "bahriya_tls_bundle" "edge" {
  handle = "edge-tls"
  name   = "Edge TLS"
  ca     = file(var.ca_path)
  cert   = file(var.cert_path)
  key    = var.tls_private_key
}
