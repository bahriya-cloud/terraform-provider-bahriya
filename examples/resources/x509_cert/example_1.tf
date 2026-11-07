variable "client_ca_pem_path" {
  type        = string
  description = "Path to the PEM-encoded client CA certificate."
}

resource "bahriya_x509_cert" "client_ca" {
  handle = "client-ca"
  name   = "Client CA"
  cert   = file(var.client_ca_pem_path)
}
