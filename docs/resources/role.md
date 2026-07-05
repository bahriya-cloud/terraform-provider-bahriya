---
page_title: "bahriya_role Resource - Bahriya"
subcategory: ""
description: |-
  A custom organisation role — a named set of permission grants across resource kinds and scopes.
---

# bahriya_role (Resource)

Manages a **custom** organisation role: a named set of permission grants that you assign to members. Each grant is a `{level, resource, permission}` triple.

System roles (`owner`, `admin`, `member`, `viewer`) are managed by Bahriya and are read-only — this resource cannot create, update, or delete them. It only defines the role; assign members to it with the Reis CLI (`reis role:assign`) or the console.

## Example Usage

### A "Deployer" role

```hcl
resource "bahriya_role" "deployer" {
  name        = "Deployer"
  description = "Manage containers; read-only on credentials."

  permissions {
    level      = "project"
    resource   = "deployables_container_http"
    permission = "create"
  }
  permissions {
    level      = "project"
    resource   = "deployables_container_http"
    permission = "update"
  }
  permissions {
    level      = "project"
    resource   = "deployables_container_http"
    permission = "delete"
  }

  permissions {
    level      = "organisation"
    resource   = "attachables_registries"
    permission = "read"
  }
}
```

## Schema

### Required

- `name` (String) Human-readable role name.
- `permissions` (Block List, min: 1) The permission grants that make up the role. Each block has:
  - `level` (String, required) Scope of the grant: `organisation` or `project`.
  - `resource` (String, required) Resource kind, e.g. `deployables_container_http`, `deployables_memcached`, `attachables_registries`, `billing`, `user`.
  - `permission` (String, required) One of `create`, `read`, `update`, `delete`.

### Optional

- `description` (String) What the role is for.

### Read-Only

- `id` (String) UUID of the role.
- `handle` (String) Machine slug, derived from the name and immutable.
- `issystem` (Boolean) Always `false` for roles created with this resource.
- `created` (String) Creation timestamp (ISO 8601).
- `updated` (String) Last-update timestamp (ISO 8601).

## Import

Roles can be imported by their UUID:

```bash
terraform import bahriya_role.deployer 7a1e2c3d-4b5f-4061-9a2b-3c4d5e6f7081
```

Importing a system role and attempting to change it will fail — system roles are read-only.
