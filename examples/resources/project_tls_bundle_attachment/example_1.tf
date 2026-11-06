resource "bahriya_tls_bundle" "api_tls" {
  # ... bahriya_tls_bundle configuration ...
}

resource "bahriya_project_tls_bundle_attachment" "web_tls_bundle" {
  project_id = bahriya_project.web.id
  handle     = bahriya_tls_bundle.api_tls.handle
}
