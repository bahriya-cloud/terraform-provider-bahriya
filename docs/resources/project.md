---
page_title: "bahriya_project Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya project — the logical grouping that owns containers, secrets, registries, and Memcached instances.
---

# bahriya_project (Resource)

Manages a Bahriya project. A project is the namespace inside an organisation that owns containers, registries, secrets, and Memcached instances.

## Example Usage

```terraform
resource "bahriya_project" "web" {
  handle  = "web-prod"
  name    = "Web Production"
  regions = ["falkenstein-1"]
}
```

### Multi-Region Project

```terraform
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

- `users` (List of String) - User IDs in this project.

### Read-Only

- `id` (String) - Project UUID.
- `organisation` (String) - Organisation UUID.
- `quotalimitcpu` (String) - Namespace CPU limit quota (platform-managed). Null when the project follows the platform default.
- `quotalimitmemory` (String) - Namespace memory limit quota (platform-managed). Null when the project follows the platform default.
- `quotarequestcpu` (String) - Namespace CPU request quota (platform-managed). Null when the project follows the platform default.
- `quotarequestmemory` (String) - Namespace memory request quota (platform-managed). Null when the project follows the platform default.
- `effectivequotalimitcpu` (String) - Namespace CPU limit quota in force, platform default resolved.
- `effectivequotalimitmemory` (String) - Namespace memory limit quota in force, platform default resolved.
- `effectivequotarequestcpu` (String) - Namespace CPU request quota in force, platform default resolved.
- `effectivequotarequestmemory` (String) - Namespace memory request quota in force, platform default resolved.

The `quota*` attributes are the project's own overrides and are null unless staff have granted the project an explicit allowance. The `effectivequota*` attributes are always populated: they are the ceilings actually enforced against every deployment in each of the project's regions, with the platform default already resolved. Read those when sizing a workload — the allowance is replicated into each region rather than divided between them, so a container spanning three regions must fit inside the same ceilings three times.

```terraform
output "cpu_headroom" {
  value = bahriya_project.web.effectivequotarequestcpu
}
```

## Import

Projects can be imported using their UUID:

```bash
terraform import bahriya_project.web 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
