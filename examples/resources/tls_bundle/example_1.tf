resource "bahriya_tls_bundle" "edge" {
  handle = "edge-tls"
  name   = "Edge TLS"
  ca     = file("${path.module}/ca.pem")
  cert   = file("${path.module}/server.pem")
  key    = file("${path.module}/server.key")
}
