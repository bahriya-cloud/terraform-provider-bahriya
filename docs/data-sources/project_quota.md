---
page_title: "bahriya_project_quota Data Source - Bahriya"
subcategory: ""
description: |-
  Reads a project's resource allowance and how much of it is in use, per region.
---

# bahriya_project_quota (Data Source)

Reads a project's resource allowance and how much of it is currently in use.

Every deployment is checked against this allowance before it is created. A container, Memcached or
Valkey instance that would take a region past its ceiling is refused, and nothing is written — no
resource, no billing, no deployment. Reading the allowance first is therefore how you size
configuration that will be accepted, rather than discovering the limit by applying and being
refused.

The allowance is read-only. Raising it is a support request, not a Terraform change.

## Per region, not per project

The allowance is **replicated into every region a project runs in**, not divided between them. Each
region gets the full ceiling, and a deployment spanning three regions has to fit inside it three
separate times. A project can therefore be full in one region and nearly empty in another, which is
why every figure here is reported per region and there is no project-wide total.

## Reservations and ceilings

Each region reports four figures, and both pairs are enforced independently:

- **reserved** — what the workloads are guaranteed.
- **ceiling** — the most they may use.

A ceiling is set above the reservation it belongs to, so the ceiling is normally what runs out
first. A project with plenty of reservation left can still be unable to accept anything.

`used` is what is reserved right now. `peak` is what the same resources would reserve if every
autoscaled workload grew to its configured maximum at the same time — the figure that decides
whether they actually can.

## Example Usage

```terraform
data "bahriya_project_quota" "app" {
  project = bahriya_project.app.id
}

output "falkenstein_cpu_headroom" {
  value = one([
    for r in data.bahriya_project_quota.app.regions : r.available.cpu_ceiling
    if r.region == "falkenstein-1"
  ])
}
```

Sizing a container against the allowance rather than hard-coding a guess:

```terraform
data "bahriya_project_quota" "app" {
  project = bahriya_project.app.id
}

locals {
  # Every region carries the same ceiling, so the first entry is representative.
  ceiling = data.bahriya_project_quota.app.regions[0].available.cpu_ceiling
}

output "project_cpu_ceiling" {
  value = local.ceiling
}
```

## Schema

### Required

- `project` (String) - Project identifier.

### Read-Only

- `handle` (String) - Project handle.
- `regions` (Attributes List) - One entry per region the project runs in, including regions with nothing deployed yet. (see [below for nested schema](#nestedatt--regions))

<a id="nestedatt--regions"></a>
### Nested Schema for `regions`

Read-Only:

- `region` (String) - Region identifier.
- `used` (Attributes) - Reserved by what is running now. (see [below for nested schema](#nestedatt--regions--amounts))
- `peak` (Attributes) - Reserved if every autoscaled workload reached its configured maximum at the same time. (see [below for nested schema](#nestedatt--regions--amounts))
- `available` (Attributes) - The ceiling. Identical in every region. (see [below for nested schema](#nestedatt--regions--amounts))

<a id="nestedatt--regions--amounts"></a>
### Nested Schema for `regions.used`, `regions.peak` and `regions.available`

Read-Only:

- `reserved_cpu` (String) - CPU guaranteed to the workloads, e.g. `1500m`.
- `reserved_memory` (String) - Memory guaranteed to the workloads, e.g. `2Gi`.
- `cpu_ceiling` (String) - The most CPU the workloads may use, e.g. `8000m`.
- `memory_ceiling` (String) - The most memory the workloads may use, e.g. `16G`.

## When a deployment is refused

If a resource would take a region past its ceiling, the apply fails with a diagnostic naming the
region, the limit and the shortfall:

```
Error: Project resource limits exceeded

  with bahriya_container_http.api,
  on main.tf line 12, in resource "bahriya_container_http" "api":
  12: resource "bahriya_container_http" "api" {

This change needs 6600m of CPU ceiling in region falkenstein-1, but project "my-project"
has only 6500m free of its 8000m ceiling. Raise a support ticket to request an increase.
  falkenstein-1 / CPU ceiling: needs 6600m, 6500m free of 8000m
```

Nothing is created when this happens, so the configuration can be adjusted and re-applied without
cleaning anything up.

A deployment that fits but cannot reach its configured `max_replicas` produces a **warning** rather
than an error — it is created and runs, and autoscaling simply stops short of the maximum. The
warning names the replica count that is actually reachable.
