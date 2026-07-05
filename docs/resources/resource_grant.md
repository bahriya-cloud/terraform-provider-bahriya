---
page_title: "bahriya_resource_grant Resource - Bahriya"
subcategory: ""
description: |-
  Shares a specific resource instance with an organisation member — a user-level, additive instance ACL.
---

# bahriya_resource_grant (Resource)

Grants one organisation member a set of permissions on **one specific resource instance** (a container, a Memcached instance, or an attachable such as a registry or secret).

Grants are **additive** — they only ever widen the member's access to that one instance; they never remove access the member already has through their role, and they affect nothing else. The member must already belong to the organisation.

Because the API has no in-place update for a grant, changing any attribute (including the permission set) forces replacement: the old grant is revoked and a new one created.

## Example Usage

### Share a container, read + update

```hcl
resource "bahriya_resource_grant" "share_api_container" {
  touser       = "5f9c1a2b-3d4e-4f60-8a1b-2c3d4e5f6071"
  resourcetype = "deployables_container_http"
  resourceid   = bahriya_container.api.id
  permissions  = ["read", "update"]
}
```

### Share a registry, read-only

```hcl
resource "bahriya_resource_grant" "share_registry" {
  touser       = var.contractor_user_id
  resourcetype = "attachables_registries"
  resourceid   = bahriya_registry.ghcr.id
  permissions  = ["read"]
}
```

## Schema

### Required

- `touser` (String) UUID of the member to share with. Must already be a member of the organisation. Changing it replaces the grant.
- `resourcetype` (String) Resource kind, e.g. `deployables_container_http`, `attachables_registries`. Changing it replaces the grant.
- `resourceid` (String) UUID of the specific instance to share. Changing it replaces the grant.
- `permissions` (Set of String) Permissions to grant on the instance: any of `create`, `read`, `update`, `delete`. Changing the set replaces the grant.

### Read-Only

- `id` (String) Synthetic identifier: `<resourcetype>|<resourceid>|<touser>`.
- `grant_ids` (List of String) The individual grant rows backing this share (one per permission).

## Import

Resource grants can be imported using the composite id `<resourcetype>|<resourceid>|<touser>`:

```bash
terraform import bahriya_resource_grant.share_registry \
  'attachables_registries|a1b2c3d4-5e6f-4071-8a2b-3c4d5e6f7081|5f9c1a2b-3d4e-4f60-8a1b-2c3d4e5f6071'
```
