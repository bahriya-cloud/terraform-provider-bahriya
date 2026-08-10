---
page_title: "bahriya_project Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya project — the logical grouping that owns containers, secrets, registries, and Memcached instances.
---

# bahriya_project (Resource)

Manages a Bahriya project. A project is the namespace inside an organisation that owns containers, registries, secrets, and Memcached instances.

## Example Usage

```hcl
resource "bahriya_project" "web" {
  handle  = "web-prod"
  name    = "Web Production"
  regions = ["falkenstein-1"]
}
```

### Multi-Region Project

```hcl
data "bahriya_regions" "active" {
  status_filter = "active"
}

resource "bahriya_project" "global" {
  handle  = "global-app"
  name    = "Global Application"
  regions = [for r in data.bahriya_regions.active.regions : r.id]
}
```

## Schema

### Required

- `handle` (String) - Unique lowercase handle. Immutable — changing this forces recreation. Handles are **not released** on delete (soft-delete).
- `name` (String) - Display name.
- `regions` (List of String) - Regions this project may deploy into. At least one is required.

### Optional

- `registries` (List of String) - Registry handles in this project.
- `secrets` (List of String) - Secret handles in this project.
- `users` (List of String) - User IDs in this project.

### Read-Only

- `id` (String) - Project UUID.
- `organisation` (String) - Organisation UUID.
- `quotalimitcpu` (String) - Namespace CPU limit quota (platform-managed).
- `quotalimitmemory` (String) - Namespace memory limit quota (platform-managed).
- `quotarequestcpu` (String) - Namespace CPU request quota (platform-managed).
- `quotarequestmemory` (String) - Namespace memory request quota (platform-managed).

## Import

Projects can be imported using their UUID:

```bash
terraform import bahriya_project.web 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
