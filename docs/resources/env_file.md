---
page_title: "bahriya_env_file Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya env file — a reusable, versioned set of KEY=VALUE pairs.
---

# bahriya_env_file (Resource)

Manages a Bahriya env file. The content is a dotenv-style document (`KEY=VALUE` per line, with `#` comments and blank lines allowed). Env files are versioned and reusable across containers.

## Example Usage

```hcl
resource "bahriya_env_file" "api" {
  handle  = "api-env"
  name    = "API env"
  content = <<-EOT
    # API runtime configuration
    LOG_LEVEL=info
    METRICS_ENABLED=true
    REGION=eu-west
  EOT
}
```

## Schema

### Required

- `handle` (String) - DNS-1123 compliant: lowercase alphanumeric and hyphens only. Immutable — changing forces recreation.
- `name` (String) - Display name.
- `content` (String) - KEY=VALUE pairs, one per line. Comment lines (`#`) and blank lines are allowed.

### Optional

- `maxversions` (Number) - Maximum number of historical versions to retain.

### Read-Only

- `id` (String) - Env file UUID.
- `billable` (Boolean) - Whether this env file is billable.
- `currentversion` (Number) - The currently active version number.
- `entry_count` (Number) - Number of KEY=VALUE entries parsed from the content.
- `managedbyresourceid` (String) - UUID of the managing resource, if any.
- `managedbyresourcetype` (String) - Type of the managing resource, if any.
- `organisation` (String) - Organisation UUID.

## Import

```bash
terraform import bahriya_env_file.api 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
