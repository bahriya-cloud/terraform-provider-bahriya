resource "bahriya_x509_cert" "client_cert" {
  # ... bahriya_x509_cert configuration ...
}

resource "bahriya_project_x509_cert_attachment" "web_x509_cert" {
  project_id = bahriya_project.web.id
  handle     = bahriya_x509_cert.client_cert.handle
}
