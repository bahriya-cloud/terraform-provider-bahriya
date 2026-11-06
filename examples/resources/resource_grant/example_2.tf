resource "bahriya_resource_grant" "share_registry" {
  to_user       = var.contractor_user_id
  resource_type = "attachables_registries"
  resource_id   = bahriya_registry.ghcr.id
  permissions   = ["read"]
}
