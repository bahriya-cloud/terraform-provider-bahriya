resource "bahriya_plain_config" "nginx_snippet" {
  handle  = "nginx-snippet"
  name    = "nginx snippet"
  content = <<-EOT
    client_max_body_size 32m;
    proxy_read_timeout 90;
  EOT
}
