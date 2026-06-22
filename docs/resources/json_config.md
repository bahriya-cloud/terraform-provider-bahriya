---
page_title: "bahriya_json_config Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya JSON config — a reusable, versioned JSON document.
---

# bahriya_json_config (Resource)

Manages a Bahriya JSON config. The content is validated as JSON on create and update. Configs are versioned and reusable across containers.

## Example Usage

```hcl
resource "bahriya_json_config" "app" {
  handle  = "app-json"
  name    = "App JSON"
  content = jsonencode({
    server = {
      port    = 8080
      timeout = "30s"
    }
    features = {
      beta = false
    }
  })
}
```

## Schema

### Required

- `handle` (String) - DNS-1123 compliant: lowercase alphanumeric and hyphens only. Immutable — changing forces recreation.
- `name` (String) - Display name.
- `content` (String) - Valid JSON content.

### Optional

- `maxversions` (Number) - Maximum number of historical versions to retain.

### Read-Only

- `id` (String) - Config UUID.
- `billable` (Boolean) - Whether this config is billable.
- `currentversion` (Number) - The currently active version number.
- `managedbyresourceid` (String) - UUID of the managing resource, if any.
- `managedbyresourcetype` (String) - Type of the managing resource, if any.
- `organisation` (String) - Organisation UUID.

## Import

```bash
terraform import bahriya_json_config.app 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
