resource "bahriya_path_rule" "internal" {
  container_id = bahriya_container.api.id
  handle       = "internal-tools"
  path         = "/internal"
  priority     = 100

  ipwhitelistenabled = true
  ipwhitelist        = ["10.0.0.0/8", "192.168.0.0/16"]

  basicauthenabled = true
  basicauthcredentials = [
    {
      username = "ops"
      password = var.ops_password
    },
  ]
}
