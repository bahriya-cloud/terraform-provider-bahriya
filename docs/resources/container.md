---
page_title: "bahriya_container Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya container — an HTTP server, background worker, or scheduled cron job deployed across one or more regions.
---

# bahriya_container (Resource)

Manages a Bahriya container workload. Containers come in three types:

| Type | Description |
|---|---|
| `http` (default) | Long-running service that accepts incoming HTTP traffic. Gets a hostname, TLS, autoscaling, rate limiting, and basic auth. |
| `worker` | Long-running background process with no public network exposure. |
| `cronjob` | Scheduled container that runs to completion on a cron expression. |

The type is set at creation time and is immutable.

## Example Usage

### HTTP Container

```hcl
resource "bahriya_container" "api" {
  handle          = "api"
  name            = "API Server"
  image           = "ghcr.io/myorg/api:1.2.3"
  containerport   = "3000"
  healthcheckpath = "/healthz"
  mincpu          = "250"
  minmemory       = "256"

  autoscalingminreplicas = "2"
  autoscalingmaxreplicas = "5"

  activeregions = ["falkenstein-1", "virginia-1"]
  project       = bahriya_project.web.id
  registry      = bahriya_registry.ghcr.handle

  newenvvar {
    key   = "NODE_ENV"
    value = "production"
  }

  secretsenvvar {
    secret = bahriya_secret.db_password.handle
    name   = "DATABASE_PASSWORD"
  }

  hostnames {
    hostname = "api.example.com"
  }
}
```

### Worker

```hcl
resource "bahriya_container" "queue" {
  handle = "queue-worker"
  name   = "Queue Worker"
  image  = "ghcr.io/myorg/app:1.0.0"
  type   = "worker"

  mincpu    = "200"
  minmemory = "256"

  autoscalingminreplicas = "1"
  activeregions          = ["falkenstein-1"]
  project                = bahriya_project.web.id

  command = ["/usr/bin/php"]
  args    = ["artisan", "queue:work", "--tries=3"]
}
```

### Cron Job

```hcl
resource "bahriya_container" "nightly" {
  handle   = "nightly-reports"
  name     = "Nightly Reports"
  image    = "ghcr.io/myorg/reports:1.0.0"
  type     = "cronjob"
  schedule = "0 2 * * *"
  timezone = "Europe/London"

  concurrencypolicy = "Forbid"

  mincpu    = "100"
  minmemory = "256"

  autoscalingminreplicas = "1"
  activeregions          = ["falkenstein-1"]
  project                = bahriya_project.web.id

  command = ["/usr/bin/php"]
  args    = ["artisan", "reports:nightly"]
}
```

### With Rate Limiting and IP Rules

```hcl
resource "bahriya_container" "api" {
  handle          = "rate-limited-api"
  name            = "Rate Limited API"
  image           = "nginx:alpine"
  containerport   = "80"
  healthcheckpath = "/"
  mincpu          = "100"
  minmemory       = "128"

  autoscalingminreplicas = "1"
  activeregions          = ["falkenstein-1"]
  project                = bahriya_project.web.id

  ratelimitingenabled          = true
  ratelimitingrequestspersecond = 10
  ratelimitingrequestsperminute = 300

  ipwhitelistenabled = true
  ipwhitelist        = ["203.0.113.0/24", "198.51.100.0/24"]
}
```

## Schema

### Required

- `handle` (String) - Unique lowercase handle. Immutable — changing this forces recreation. Handles are **not released** on delete (soft-delete).
- `name` (String) - Display name.
- `image` (String) - Container image reference including tag.
- `mincpu` (String) - Minimum CPU in millicores.
- `minmemory` (String) - Minimum memory in megabytes.
- `activeregions` (List of String) - Active regions. At least one required.

### Optional

- `type` (String) - Container type: `http` (default), `worker`, or `cronjob`.
- `containerport` (String) - Port the container listens on. **Required for HTTP containers** — deploy will fail without it.
- `healthcheckpath` (String) - Health check endpoint path. **Required for HTTP containers** — the platform probes this path to decide if the container is healthy.
- `project` (String) - Project UUID. Changing this forces recreation.
- `registry` (String) - Registry handle for private images.
- `autoscalingminreplicas` (String) - Minimum replicas.
- `autoscalingmaxreplicas` (String) - Maximum replicas (enables autoscaling).
- `autoscalingtargetcpu` (String) - CPU utilisation target for autoscaling.
- `autoscalingtargetmemory` (String) - Memory utilisation target for autoscaling.
- `autoscalingenabled` (Boolean) - Whether autoscaling is enabled.
- `command` (List of String) - Override container ENTRYPOINT.
- `args` (List of String) - Override container CMD arguments.
- `bootstraptime` (Number) - Expected application bootstrap time in seconds, used for startup probe timing.
- `ephemeralstoragegb` (Number) - Ephemeral storage in GB per pod.
- `externalnetworkingenabled` (Boolean) - Expose the container externally via ingress.
- `defaulthostname` (String) - Host alias for per-region default hostnames.
- `customdnsmode` (String) - Custom DNS mode.
- `dnsfailoverenabled` (Boolean) - Enable DNS failover.
- `dnsttl` (Number) - DNS TTL in seconds for vanity records.

**Observability (HTTP + worker):**

- `prometheusport` (String) - Port exposing Prometheus metrics.
- `prometheuspath` (String) - Path to Prometheus metrics endpoint (default `/metrics`).

**Security (HTTP only):**

- `basicauthenabled` (Boolean) - Enable basic authentication on the container ingress.
- `ratelimitingenabled` (Boolean) - Enable rate limiting by IP.
- `ratelimitingrequestspersecond` (Number) - Max requests per second per IP.
- `ratelimitingrequestsperminute` (Number) - Max requests per minute per IP.
- `ratelimitingrequestsperhour` (Number) - Max requests per hour per IP.
- `ipwhitelistenabled` (Boolean) - Enable IP allow-list.
- `ipwhitelist` (List of String) - Allowed IP addresses/CIDRs.
- `ipblacklistenabled` (Boolean) - Enable IP deny-list.
- `ipblacklist` (List of String) - Denied IP addresses/CIDRs.

**Proxy cache (HTTP only):**

- `proxycacheenabled` (Boolean) - Enable proxy cache backed by managed Memcached.
- `proxycachettl` (Number) - Seconds the plugin serves the cached entry.
- `proxycachestoragettl` (Number) - Seconds Memcached keeps the entry.
- `proxycachesizemb` (Number) - Total cache cluster size in MB (multiple of 256).
- `proxycachemaxitemsizemb` (Number) - Max size of any single cached item.
- `proxycachevariant` (String) - `pre-rate-limit` or `post-rate-limit`.
- `proxycachecontenttypes` (List of String) - Response content types eligible for caching.
- `proxycacherequestmethods` (List of String) - HTTP methods eligible for caching.
- `proxycacheresponsecodes` (List of Number) - Response status codes eligible for caching.
- `proxycachevaryheaders` (List of String) - Request headers that contribute to the cache key.
- `proxycachevaryqueryparams` (List of String) - Query params that contribute to the cache key.
- `proxycachevarybodyjsonfields` (List of String) - JSON body fields that contribute to the cache key.
- `proxycachehonorcachecontrol` (Boolean) - Honour RFC 7234 Cache-Control directives.
- `proxycacheallowforcecacheheader` (Boolean) - Allow `X-Proxy-Cache-Memcached-Force: true` header to bypass checks.

**Schedule (cronjob only):**

- `schedule` (String) - 5-field cron expression. **Required for cron jobs.**
- `timezone` (String) - IANA timezone the schedule fires in (default `UTC`).
- `concurrencypolicy` (String) - `Allow`, `Forbid` (default), or `Replace`.
- `suspended` (Boolean) - Pause scheduling without deleting the cron job.
- `backofflimit` (Number) - Max retry attempts per execution.
- `activedeadlineseconds` (Number) - Hard wall-clock cap per execution in seconds.
- `ttlsecondsafterfinished` (Number) - Seconds to keep finished run pods for log inspection.
- `startingdeadlineseconds` (Number) - Skip a run if more than N seconds late.
- `successfuljobshistorylimit` (Number) - How many successful run records to keep.
- `failedjobshistorylimit` (Number) - How many failed run records to keep.

### Blocks

<a id="nestedblock--newenvvar"></a>
#### `newenvvar` (Block List)

Environment variable key-value pairs.

- `key` (String, Required) - Variable name.
- `value` (String, Required) - Variable value.

<a id="nestedblock--secretsenvvar"></a>
#### `secretsenvvar` (Block List)

Inject a secret as an environment variable.

- `name` (String, Required) - Environment variable name inside the container.
- `secret` (String, Required) - Secret handle to inject.

<a id="nestedblock--hostnames"></a>
#### `hostnames` (Block List)

Custom hostnames attached to this container.

- `hostname` (String, Required) - The hostname (e.g. `api.example.com`).
- `wwwredirect` (Boolean, Optional) - Redirect `www.` prefix to the bare domain.

<a id="nestedblock--basicauthcredentials"></a>
#### `basicauthcredentials` (Block List)

Basic auth credentials (up to 10). HTTP containers only.

- `username` (String, Required) - Username.
- `password` (String, Required) - Password.

<a id="nestedblock--initjobs"></a>
#### `initjobs` (Block List)

Init jobs that run to completion before the main container starts. HTTP containers only.

- `handle` (String, Required) - Init job handle.
- `image` (String, Required) - Container image.
- `command` (List of String, Optional) - Override ENTRYPOINT.
- `args` (List of String, Optional) - Override CMD arguments.
- `mincpu` (String, Optional) - CPU in millicores.
- `minmemory` (String, Optional) - Memory in megabytes.
- `registry` (String, Optional) - Registry handle for private images.

<a id="nestedblock--persistentvolumes"></a>
#### `persistentvolumes` (Block List)

Persistent volumes. Each replica gets its own copy. Containers with persistent volumes cannot use autoscaling.

- `handle` (String, Required) - Volume handle.
- `mountpath` (String, Required) - Mount path inside the container.
- `sizegb` (Number, Required) - Volume size in GB.

### Read-Only

- `id` (String) - Container UUID.
- `status` (String) - Current container status.
- `organisation` (String) - Organisation UUID.
- `maxcpu` (String) - Computed CPU limit.
- `maxmemory` (String) - Computed memory limit.
- `projectname` (String) - Project display name.
- `vanityhostname` (String) - Platform-assigned vanity hostname.
- `proxycachememcached` (String) - ID of the managed Memcached instance backing the proxy cache.

## Import

Containers can be imported using their UUID:

```bash
terraform import bahriya_container.api 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
