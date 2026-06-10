---
page_title: "bahriya_organisation Data Source - Bahriya"
subcategory: ""
description: |-
  Fetches the current organisation configured in the provider.
---

# bahriya_organisation (Data Source)

Fetches details about the current organisation, as identified by the `organisation_id` in the provider configuration.

## Example Usage

```hcl
data "bahriya_organisation" "current" {}

output "org_name" {
  value = data.bahriya_organisation.current.name
}

output "org_handle" {
  value = data.bahriya_organisation.current.handle
}
```

## Schema

### Read-Only

- `id` (String) - Organisation UUID.
- `name` (String) - Organisation display name.
- `handle` (String) - Organisation handle (permanent, unique identifier).
- `email` (String) - Organisation contact email.
- `role` (String) - Authenticated user's role in this organisation.
