resource "bahriya_role" "deployer" {
  name        = "Deployer"
  description = "Manage containers; read-only on credentials."

  permissions = [
    {
      level      = "project"
      resource   = "deployables_container_http"
      permission = "create"
    },
    {
      level      = "project"
      resource   = "deployables_container_http"
      permission = "update"
    },
    {
      level      = "project"
      resource   = "deployables_container_http"
      permission = "delete"
    },
    {
      level      = "organisation"
      resource   = "attachables_registries"
      permission = "read"
    },
  ]
}
