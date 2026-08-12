---
page_title: "bahriya_valkey Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya Valkey instance — a managed Valkey datastore for caching or persistent key-value storage.
---

# bahriya_valkey (Resource)

Manages a Bahriya Valkey instance. Valkey instances provide a managed, Redis-compatible datastore that containers can connect to, with optional persistence, high availability, sharding and external exposure.

Two axes shape an instance:

- `purpose` — `cache` (no persistence, `allkeys-lru` eviction) or `store` (RDB+AOF persistence, `noeviction`, requires `storagegb`). Immutable after creation.
- `tier` — `single` (one node), `ha` (Sentinel topology; clients need a Sentinel-aware driver) or `sharded` (cluster topology; clients need a cluster-aware driver). The `size` axis (`standard` or `hardened`) applies to `ha` and `sharded` and controls the replica count per shard.

Creating a Valkey instance is asynchronous — the provider waits up to 20 minutes for the instance to reach `running` status. Destroying waits up to 20 minutes for `terminated` status. If a create does not settle in time, the instance is saved to state with its current status and a warning; run `terraform apply` again to re-check.

The connection password is never returned by the API. Set one explicitly with `password`, or omit it and reveal the generated password once from the console.

## Example Usage

### Basic Cache

```hcl
resource "bahriya_valkey" "sessions" {
  handle        = "session-cache"
  name          = "Session Cache"
  purpose       = "cache"
  tier          = "single"
  memorymb      = 256
  activeregions = ["falkenstein-1"]
  project       = bahriya_project.web.id
}
```

### Highly Available Store with Backups

```hcl
resource "bahriya_valkey" "queue" {
  handle        = "job-queue"
  name          = "Job Queue"
  purpose       = "store"
  tier          = "ha"
  size          = "standard"
  memorymb      = 1024
  storagegb     = 10
  activeregions = ["falkenstein-1"]
  project       = bahriya_project.web.id

  backupenabled  = true
  backupschedule = "0 3 * * *"
}
```

### Externally Exposed with TLS

External exposure requires TLS, auth and a non-empty CIDR allowlist — the API rejects any state where `externalenabled` is true and one of the three is missing.

```hcl
resource "bahriya_valkey" "shared" {
  handle        = "shared-cache"
  name          = "Shared Cache"
  purpose       = "cache"
  tier          = "single"
  memorymb      = 512
  activeregions = ["falkenstein-1"]
  project       = bahriya_project.web.id

  externalenabled = true
  tlsenabled      = true
  authenabled     = true
  allowedips      = ["198.51.100.0/24"]

  hostnames = [
    { hostname = "cache.example.com" },
  ]
}
```

With `tlsenabled = true` and no `tlsbundle`, the platform issues and manages a TLS bundle in your vault. To serve your own bundle instead, set `tlsbundle` to its handle — it must cover every hostname the instance answers to, and the API returns a 400 listing any missing names.

### Sharded Cluster with Configuration Tuning

```hcl
resource "bahriya_valkey" "catalog" {
  handle        = "catalog"
  name          = "Catalog"
  purpose       = "cache"
  tier          = "sharded"
  size          = "hardened"
  shards        = 3
  memorymb      = 2048
  activeregions = ["falkenstein-1", "singapore-1"]
  project       = bahriya_project.web.id

  maxmemorypolicy = "allkeys-lfu"

  config = {
    "tcp-keepalive" = "300"
    "timeout"       = "0"
    "databases"     = "16"
    "loglevel"      = "notice"
  }
}
```

`shards` is grow-only in self-service: raising it is automated, lowering it is an operator procedure raised through a support ticket.

## Schema

Nested items (`hostnames`) are Attributes Lists — assign them with attribute syntax (`hostnames = [{ hostname = "..." }]`), not block syntax.

### Required

- `handle` (String) - Unique lowercase handle. Immutable — changing this forces recreation. Handles are **not released** on delete.
- `name` (String) - Display name.
- `purpose` (String) - Sets the defaults bundle: `cache` = no persistence, allkeys-lru; `store` = RDB+AOF persistence, noeviction, storage required. Immutable after creation — changing this forces recreation.
- `tier` (String) - Product tier. `single` = one node, restarts from disk. `ha` = Sentinel topology, needs a Sentinel-aware client driver. `sharded` = cluster topology, needs a cluster-aware client driver.
- `memorymb` (Number) - Memory per node in MB. Billed per node.
- `activeregions` (List of String) - Regions this instance deploys into. At least one is required. Each region runs an independent copy with the same configuration and its own data.

### Optional

- `project` (String) - Project UUID. Changing this forces recreation.
- `size` (String) - Size axis for the `ha` and `sharded` tiers (`standard` is the default). Not applicable to `single`.
- `shards` (Number) - Sharded tier only. Grow-only in self-service: raising it is automated, lowering it is an operator procedure raised through a support ticket. Minimum 3.
- `storagegb` (Number) - Persistent storage per node in GB. Required for purpose `store`, not applicable to `cache`.
- `maxmemorypolicy` (String) - Eviction policy. Defaults from purpose (`cache` = allkeys-lru, `store` = noeviction) and remains freely settable.
- `config` (Map of String) - Allowlisted Valkey configuration keys (`tcp-keepalive`, `timeout`, `tcp-backlog`, `databases`, `loglevel`). Unknown keys are rejected naming the key. Values are given as strings; numeric values are normalised server-side.
- `authenabled` (Boolean) - Require the connection password (default true).
- `password` (String, Sensitive) - Optional. Omitted on create, the platform generates one. Never returned; reveal it once from the console.
- `tlsenabled` (Boolean) - Serve TLS (default false).
- `tlsbundle` (String) - Handle of a customer TLS bundle to serve. Omitted with TLS enabled, the platform issues and manages a bundle in your vault. A supplied bundle must cover every hostname the instance answers to; a 400 lists any missing names.
- `defaulthostname` (String) - Server-generated default hostname label. Omitted on create, the platform assigns one; immutable thereafter. Only required up front when supplying a customer `tlsbundle`, so SAN coverage can be validated.
- `externalenabled` (Boolean) - Expose the instance at the platform edge on port 6379. Requires TLS, auth and a non-empty IP allowlist.
- `allowedips` (List of String) - CIDR allowlist for external access. Required non-empty while external is enabled.
- `hostnames` (Attributes List) - Custom hostnames. A custom hostname on TCP is a vanity name, not a trust upgrade — clients trust the instance CA either way. (see [below for nested schema](#nestedatt--hostnames))
- `backupenabled` (Boolean) - Daily backups. Store purpose only.
- `backupschedule` (String) - Backup cron schedule. Retention is fixed at 7 daily snapshots.
- `networkpolicies` (List of String) - Network policies narrowed to this Valkey instance, referenced by handle. Datastores are L3/L4 only — an L7 policy applied here enforces its L3/L4 rules.

### Read-Only

- `id` (String) - Valkey instance UUID.
- `status` (String) - Current instance status.
- `nodes` (Number) - Derived node count per region for the current tier, size and shards.
- `organisation` (String) - Organisation UUID.
- `projectname` (String) - Project display name.

<a id="nestedatt--hostnames"></a>
#### Nested Schema for `hostnames` (Attributes List)

- `hostname` (String, Required) - Custom hostname (FQDN). Point a CNAME at the instance's per-region hostname. Lowercased and trimmed server-side.

## Import

Valkey instances can be imported using their UUID:

```bash
terraform import bahriya_valkey.sessions 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
