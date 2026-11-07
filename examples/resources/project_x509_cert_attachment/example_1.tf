variable "client_cert" {
  type = string
}

resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_x509_cert" "client_cert" {
  handle = "client-cert"
  name   = "Client certificate"
  cert   = var.client_cert
}

resource "bahriya_project_x509_cert_attachment" "web_x509_cert" {
  project_id = bahriya_project.web.id
  handle     = bahriya_x509_cert.client_cert.handle
}
