resource "bahriya_path_rule" "admin" {
  container_id = bahriya_container.api.id
  handle       = "admin"
  path         = "/api/admin"

  basicauthenabled = true
  basicauthcredentials = [
    {
      username = "alice"
      password = var.admin_password
    },
  ]
}
