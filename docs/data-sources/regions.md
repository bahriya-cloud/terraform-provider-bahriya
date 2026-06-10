---
page_title: "bahriya_regions Data Source - Bahriya"
subcategory: ""
description: |-
  Lists all available Bahriya regions, with optional status filtering.
---

# bahriya_regions (Data Source)

Lists all available Bahriya regions. Use `status_filter` to return only regions with a specific status.

## Example Usage

### All Active Regions

```hcl
data "bahriya_regions" "active" {
  status_filter = "active"
}

output "region_ids" {
  value = [for r in data.bahriya_regions.active.regions : r.id]
}
```

### Use in a Project

```hcl
data "bahriya_regions" "active" {
  status_filter = "active"
}

resource "bahriya_project" "web" {
  handle  = "web-prod"
  name    = "Web Production"
  regions = [data.bahriya_regions.active.regions[0].id]
}
```

### All Regions (No Filter)

```hcl
data "bahriya_regions" "all" {}
```

## Schema

### Optional

- `status_filter` (String) - Only return regions with this status (e.g. `active`). If omitted, all regions are returned.

### Read-Only

- `regions` (List of Object) - List of regions matching the filter.

Each region object contains:

- `id` (String) - Region identifier.
- `name` (String) - Region display name.
- `description` (String) - Human-readable region description.
- `class` (String) - Region class.
- `status` (String) - Region status.
- `city` (String) - City.
- `state` (String) - State or province.
- `country` (String) - Country.
- `latitude` (Number) - Geographic latitude.
- `longitude` (Number) - Geographic longitude.
