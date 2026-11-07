variable "api_ca" {
  type = string
}

variable "api_cert" {
  type = string
}

variable "api_key" {
  type      = string
  sensitive = true
}

resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_tls_bundle" "api_tls" {
  handle = "api-tls"
  name   = "API TLS"
  ca     = var.api_ca
  cert   = var.api_cert
  key    = var.api_key
}

resource "bahriya_project_tls_bundle_attachment" "web_tls_bundle" {
  project_id = bahriya_project.web.id
  handle     = bahriya_tls_bundle.api_tls.handle
}
