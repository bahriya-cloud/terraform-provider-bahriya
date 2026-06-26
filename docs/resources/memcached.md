---
page_title: "bahriya_memcached Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya Memcached instance — a managed cache backend attachable to containers.
---

# bahriya_memcached (Resource)

Manages a Bahriya Memcached instance. Memcached instances provide a managed cache backend that containers can connect to.

Creating a Memcached instance is asynchronous — the provider waits up to 10 minutes for the instance to reach `running` status. Destroying waits up to 10 minutes for `terminated` status.

## Example Usage

### Basic Instance

```hcl
resource "bahriya_memcached" "session" {
  handle        = "session-cache"
  name          = "Session Cache"
  memorymb      = 512
  activeregions = ["falkenstein-1"]
  project       = bahriya_project.web.id
}
```

### Tuned Instance

```hcl
resource "bahriya_memcached" "api_cache" {
  handle         = "api-cache"
  name           = "API Response Cache"
  memorymb       = 1024
  nodes          = 3
  maxconnections = 2048
  threads        = 8
  maxitemsizemb  = 5
  activeregions  = ["falkenstein-1"]
  project        = bahriya_project.web.id
}
```

## Schema

### Required

- `handle` (String) - Unique lowercase handle. Immutable — changing this forces recreation. Handles are **not released** on delete (soft-delete).
- `name` (String) - Display name.
- `memorymb` (Number) - Cache size in megabytes. Note: this is an integer, not a string.
- `activeregions` (List of String) - Active regions. At least one required.

### Optional

- `project` (String) - Project UUID. Changing this forces recreation.
- `nodes` (Number) - Number of nodes, 1–9 (default 1).
- `maxconnections` (Number) - Maximum concurrent connections per node (default 1024).
- `threads` (Number) - Worker threads per node (default 4).
- `maxitemsizemb` (Number) - Largest item the cache will accept in MB, max 128 (default 1).

### Read-Only

- `id` (String) - Memcached instance UUID.
- `status` (String) - Current instance status.
- `organisation` (String) - Organisation UUID.
- `projectname` (String) - Project display name.
- `managedbycontainer` (String) - When set, this instance is managed by a container's proxy cache feature and cannot be modified directly.

## Import

Memcached instances can be imported using their UUID:

```bash
terraform import bahriya_memcached.session 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
