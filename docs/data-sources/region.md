---
page_title: "bahriya_region Data Source - Bahriya"
subcategory: ""
description: |-
  Fetches details for a single Bahriya region by ID.
---

# bahriya_region (Data Source)

Fetches details for a single Bahriya region by its identifier.

## Example Usage

```hcl
data "bahriya_region" "helsinki" {
  id = "helsinki-1"
}

output "region_name" {
  value = data.bahriya_region.helsinki.name
}

output "region_country" {
  value = data.bahriya_region.helsinki.country
}
```

## Schema

### Required

- `id` (String) - Region identifier (e.g. `helsinki-1`, `falkenstein-1`, `virginia-1`).

### Read-Only

- `name` (String) - Region display name.
- `description` (String) - Human-readable region description.
- `class` (String) - Region class (e.g. `standard`).
- `status` (String) - Region status (e.g. `active`).
- `city` (String) - City where the region is located.
- `state` (String) - State or province.
- `country` (String) - Country.
- `latitude` (Number) - Geographic latitude.
- `longitude` (Number) - Geographic longitude.
