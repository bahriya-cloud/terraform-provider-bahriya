---
page_title: "bahriya_plain_config Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya plain-text config — a reusable, versioned arbitrary text document.
---

# bahriya_plain_config (Resource)

Manages a Bahriya plain-text config. The content is stored verbatim (no parsing or validation) — useful for INI files, nginx snippets, scripts, or any other text payload that must be mounted into a container.

## Example Usage

```hcl
resource "bahriya_plain_config" "nginx_snippet" {
  handle  = "nginx-snippet"
  name    = "nginx snippet"
  content = <<-EOT
    client_max_body_size 32m;
    proxy_read_timeout 90;
  EOT
}
```

## Schema

### Required

- `handle` (String) - DNS-1123 compliant: lowercase alphanumeric and hyphens only. Immutable — changing forces recreation.
- `name` (String) - Display name.
- `content` (String) - Arbitrary text content.

### Optional

- `maxversions` (Number) - Maximum number of historical versions to retain.

### Read-Only

- `id` (String) - Config UUID.
- `billable` (Boolean) - Whether this config is billable.
- `contentlength` (Number) - Length of the content in bytes.
- `currentversion` (Number) - The currently active version number.
- `managedbyresourceid` (String) - UUID of the managing resource, if any.
- `managedbyresourcetype` (String) - Type of the managing resource, if any.
- `organisation` (String) - Organisation UUID.

## Import

```bash
terraform import bahriya_plain_config.nginx_snippet 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
