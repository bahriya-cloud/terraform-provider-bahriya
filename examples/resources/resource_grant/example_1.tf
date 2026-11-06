resource "bahriya_resource_grant" "share_api_container" {
  to_user       = "5f9c1a2b-3d4e-4f60-8a1b-2c3d4e5f6071"
  resource_type = "deployables_container_http"
  resource_id   = bahriya_container.api.id
  permissions   = ["read", "update"]
}
