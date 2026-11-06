---
page_title: "bahriya_yaml_config Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya YAML config — a reusable, versioned YAML document.
---

# bahriya_yaml_config (Resource)

Manages a Bahriya YAML config. The content is validated as YAML on create and update. Configs are versioned and reusable across containers.

## Example Usage

```hcl
resource "bahriya_yaml_config" "app" {
  handle  = "app-yaml"
  name    = "App YAML"
  content = <<-EOT
    server:
      port: 8080
      timeout: 30s
    features:
      beta: false
  EOT
}
```

## Changing the content

New content is a rotation, which this provider version does not perform. Changing `content` fails at plan time with "Rotation is not supported by this provider version". Rotate the config with `reis yaml_config:rotate` or in the console, then set the new content in your configuration. Renaming (`name`) is applied in place. Terraform provider v1 rotates the content for you.

## Schema

### Required

- `handle` (String) - DNS-1123 compliant: lowercase alphanumeric and hyphens only. Immutable — changing forces recreation.
- `name` (String) - Display name.
- `content` (String) - Valid YAML content.

### Optional

- `maxversions` (Number) - Maximum number of historical versions to retain.

### Read-Only

- `id` (String) - Config UUID.
- `billable` (Boolean) - Whether this config is billable.
- `currentversion` (Number) - The currently active version number.
- `managedbyresourceid` (String) - UUID of the managing resource, if any.
- `managedbyresourcetype` (String) - Type of the managing resource, if any.
- `organisation` (String) - Organisation UUID.
- `versioncount` (Number) - How many versions are kept, the current one included.

## Import

```bash
terraform import bahriya_yaml_config.app 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
