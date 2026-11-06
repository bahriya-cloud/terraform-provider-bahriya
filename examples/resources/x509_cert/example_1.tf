resource "bahriya_x509_cert" "client_ca" {
  handle = "client-ca"
  name   = "Client CA"
  cert   = file("${path.module}/client-ca.pem")
}
