---
page_title: "bahriya_container Resource - Bahriya"
subcategory: ""
description: |-
  A container: an HTTP service, a background worker or a scheduled job, run from your image in the regions you choose.
---

# bahriya_container (Resource)

A container runs your image in every region of `regions`, inside a project. `type` sets what kind of container it is, and cannot be changed after creation:

| Type             | What it is                                                                                                                         |
|------------------|------------------------------------------------------------------------------------------------------------------------------------|
| `http` (default) | A long-running service that answers HTTP requests. It gets a hostname and TLS, and can use autoscaling, rate limiting, IP lists and basic authentication. |
| `worker`         | A long-running background process that is not reachable from the internet.                                                        |
| `cronjob`        | A job that runs to completion on the schedule in `schedule`.                                                                       |

Some settings depend on the type:

- An `http` container must state `networking.external`: `true` serves it at its hostnames, `false` keeps it reachable only by the project's other workloads. There is no default, so a hostname is published only when the configuration says so. Its `workload.port` defaults to 8080 and its `workload.healthcheck` to `/healthz`.
- A `worker` or `cronjob` container is never reachable from the internet, so it cannot set hostnames, `networking.external`, rate limiting, IP lists, basic authentication or init jobs.
- A `cronjob` also cannot set `workload.port`, `workload.healthcheck`, the Prometheus settings, `scaling.autoscaling` or `scaling.max_replicas`.

A plan that breaks one of these rules fails and names the attribute. Each attribute's description below says which types it applies to.

The settings are grouped the same way as in a Reis YAML file: `workload` (image, size, command and environment), `scaling`, `networking`, `security`, `storage`, `configs`, `vault`, `schedule` and the rest.

Creating, changing and terminating a container is asynchronous. The provider waits up to 20 minutes for the container to reach `running`, or `terminated` on delete. If a create does not settle in time, the container is saved to state with its current status and a warning; run `terraform apply` again to check it.

Container handles are not released when a container is terminated, so a handle cannot be reused.

## Example Usage

### HTTP container

```terraform
variable "ghcr_username" {
  type = string
}

variable "ghcr_token" {
  type      = string
  sensitive = true
}

variable "db_password" {
  type      = string
  sensitive = true
}

variable "tls_ca" {
  type = string
}

variable "tls_cert" {
  type = string
}

variable "tls_key" {
  type      = string
  sensitive = true
}

resource "bahriya_project" "web" {
  name    = "Web"
  handle  = "web"
  regions = ["falkenstein-1", "virginia-1"]
}

resource "bahriya_registry" "ghcr" {
  name     = "GitHub Container Registry"
  handle   = "ghcr"
  server   = "ghcr.io"
  username = var.ghcr_username
  password = var.ghcr_token
}

resource "bahriya_secret" "db_password" {
  name   = "Database password"
  handle = "db-password"
  value  = var.db_password
}

resource "bahriya_tls_bundle" "edge" {
  name   = "Edge certificate"
  handle = "edge"
  ca     = var.tls_ca
  cert   = var.tls_cert
  key    = var.tls_key
}

resource "bahriya_network_policy" "web_tier" {
  name   = "Web tier"
  handle = "web-tier"
}

resource "bahriya_container" "api" {
  name    = "API Server"
  handle  = "api"
  project = bahriya_project.web.id
  regions = ["falkenstein-1", "virginia-1"]

  workload = {
    image       = "ghcr.io/myorg/api:1.2.3"
    registry    = bahriya_registry.ghcr.id
    port        = 3000
    healthcheck = "/healthz"
    cpu         = 250
    memory      = 256

    env = {
      "NODE_ENV" = "production"
    }

    # Environment variable name = secret handle.
    secrets = {
      "DATABASE_PASSWORD" = bahriya_secret.db_password.handle
    }
  }

  scaling = {
    replicas     = 2
    max_replicas = 5
  }

  networking = {
    # Required for an HTTP container: true serves it at its hostnames.
    external = true
    domains = [
      { hostname = "api.example.com" },
    ]

    # A network policy listed here applies to this container only.
    network_policies = [bahriya_network_policy.web_tier.handle]
  }

  # Vault and config items attached to the project are mounted as files.
  vault = {
    tls_bundles = [
      { handle = bahriya_tls_bundle.edge.handle, mount_path = "/etc/tls" },
    ]
  }
}
```

### Worker

```terraform
resource "bahriya_project" "web" {
  name    = "Web"
  handle  = "web"
  regions = ["falkenstein-1"]
}

resource "bahriya_container" "queue" {
  name    = "Queue Worker"
  handle  = "queue-worker"
  type    = "worker"
  project = bahriya_project.web.id
  regions = ["falkenstein-1"]

  workload = {
    image   = "ghcr.io/myorg/app:1.0.0"
    cpu     = 200
    memory  = 256
    command = ["/usr/bin/php"]
    args    = ["artisan", "queue:work", "--tries=3"]
  }

  scaling = {
    replicas = 1
  }
}
```

### Scheduled job

```terraform
resource "bahriya_project" "web" {
  name    = "Web"
  handle  = "web"
  regions = ["falkenstein-1"]
}

resource "bahriya_container" "nightly" {
  name    = "Nightly Reports"
  handle  = "nightly-reports"
  type    = "cronjob"
  project = bahriya_project.web.id
  regions = ["falkenstein-1"]

  workload = {
    image   = "ghcr.io/myorg/reports:1.0.0"
    cpu     = 100
    memory  = 256
    command = ["/usr/bin/php"]
    args    = ["artisan", "reports:nightly"]
  }

  scaling = {
    replicas = 1
  }

  schedule = {
    schedule    = "0 2 * * *"
    tz          = "Europe/London"
    concurrency = "Forbid"
  }
}
```

### Choosing the protocol

`workload.protocol` says what the container speaks on `workload.port`. The same setting decides both how its health is checked and how requests reach it, so the two can never disagree.

The default, `http`, checks health with a GET request to `workload.healthcheck`. Use `https` when the container serves TLS on its own port; its certificate is not verified, so a self-signed certificate is acceptable. Use `tcp` when the container does not speak HTTP: the health check then only confirms that the port accepts a connection, and `workload.healthcheck` is not used.

```terraform
resource "bahriya_project" "web" {
  name    = "Web"
  handle  = "web"
  regions = ["falkenstein-1"]
}

# Terminates TLS itself, so the health check probes over HTTPS. The
# certificate is not verified, so a self-signed certificate is acceptable.
resource "bahriya_container" "secure_api" {
  name    = "Secure API"
  handle  = "secure-api"
  project = bahriya_project.web.id
  regions = ["falkenstein-1"]

  workload = {
    image       = "ghcr.io/org/secure-api:2.0.0"
    port        = 8443
    healthcheck = "/healthz"
    protocol    = "https"
    cpu         = 100
    memory      = 128
  }

  scaling = {
    replicas = 1
  }

  networking = {
    external = true
  }
}

# Speaks a binary protocol rather than HTTP, so only the port is probed.
resource "bahriya_container" "queue_bridge" {
  name    = "Queue Bridge"
  handle  = "queue-bridge"
  project = bahriya_project.web.id
  regions = ["falkenstein-1"]

  workload = {
    image    = "registry.example.com/team/bridge:1.4.2"
    port     = 9000
    protocol = "tcp"
    cpu      = 50
    memory   = 128
  }

  scaling = {
    replicas = 1
  }

  # Reached only by the project's other workloads, not from the internet.
  networking = {
    external = false
  }
}
```

### Rate limiting and IP lists

```terraform
resource "bahriya_project" "web" {
  name    = "Web"
  handle  = "web"
  regions = ["falkenstein-1"]
}

resource "bahriya_container" "api" {
  name    = "Rate Limited API"
  handle  = "rate-limited-api"
  project = bahriya_project.web.id
  regions = ["falkenstein-1"]

  workload = {
    image       = "nginx:alpine"
    port        = 80
    healthcheck = "/"
    cpu         = 100
    memory      = 128
  }

  scaling = {
    replicas = 1
  }

  networking = {
    external = true
  }

  security = {
    rate_limiting = {
      enabled    = true
      per_second = 10
      per_minute = 300
    }

    ip_allowlist = {
      enabled = true
      ips     = ["203.0.113.0/24", "198.51.100.0/24"]
    }
  }
}
```

To apply these controls to one URL prefix only, use `bahriya_path_rule`.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `handle` (String) Short name that identifies the container in hostnames and references. Lowercase letters, digits and hyphens only.
- `name` (String) Display name of the container, up to 100 characters.
- `regions` (List of String) Regions this container deploys into. At least one is required.
- `workload` (Attributes) (see [below for nested schema](#nestedatt--workload))

### Optional

- `configs` (Attributes) (see [below for nested schema](#nestedatt--configs))
- `init_jobs` (Attributes List) Init jobs that run to completion before the main container starts. HTTP containers only. Up to a configurable maximum (default 3). Init jobs run within the main container's reserved CPU and memory and are not billed separately. Not allowed on cronjob or worker containers. (see [below for nested schema](#nestedatt--init_jobs))
- `networking` (Attributes) (see [below for nested schema](#nestedatt--networking))
- `observability` (Attributes) (see [below for nested schema](#nestedatt--observability))
- `project` (String) Identifier (UUID) of the project the container belongs to. It is set when the container is created and cannot be changed.
- `proxy_cache` (Attributes) (see [below for nested schema](#nestedatt--proxy_cache))
- `scaling` (Attributes) (see [below for nested schema](#nestedatt--scaling))
- `schedule` (Attributes) (see [below for nested schema](#nestedatt--schedule))
- `security` (Attributes) (see [below for nested schema](#nestedatt--security))
- `storage` (Attributes) (see [below for nested schema](#nestedatt--storage))
- `type` (String) Container type. HTTP containers answer requests at their hostnames; workers run continuously without access from the internet; cron jobs run on a schedule. Defaults to `"http"`.
- `vault` (Attributes) (see [below for nested schema](#nestedatt--vault))

### Read-Only

- `created` (String) When the container was created (UTC).
- `id` (String) Unique identifier (UUID) of the container, assigned by the platform.
- `organisation` (String) Identifier (UUID) of the organisation that owns the container.
- `project_name` (String) Name of the project the container belongs to.
- `restarted_at` (String) When the container was last restarted on request (UTC). Null if it has never been restarted.
- `status` (String) Lifecycle state of the container: provisioning, running, terminating, terminated or error.
- `updated` (String) When the container was last changed (UTC).

<a id="nestedatt--workload"></a>
### Nested Schema for `workload`

Required:

- `cpu` (Number) CPU requested for each replica, in millicores (1000 millicores is one vCPU).
- `image` (String) The container image to deploy, including the registry host when it is not Docker Hub. An explicit tag or digest is required — an untagged reference is rejected.
- `memory` (Number) Memory requested for each replica, in MiB.

Optional:

- `args` (List of String) Override the container image CMD arguments.
- `bootstrap_time` (Number) Expected application start-up time in seconds, used to time the start-up health checks. It is 30 when not set. Defaults to `30`.
- `command` (List of String) Override the container image CMD entrypoint.
- `env` (Map of String) Environment variables set in the container, as name and value pairs.
- `healthcheck` (String) Path of the health check endpoint. Defaults to `"/healthz"` for http containers. Not allowed on cronjob containers.
- `port` (Number) Port number the container listens on. Defaults to `8080` for http containers. Not allowed on cronjob containers.
- `protocol` (String) The protocol this container speaks on its port. It decides both how the container's health is checked and how requests reach it. `http` is the default. `https` is for a container that serves TLS on its own port: health checks and requests reach it over TLS, and its certificate is not verified. `tcp` only checks that the port accepts a connection and ignores the health check path; it suits a workload that does not speak HTTP. Defaults to `"http"`.
- `registry` (String) Identifier (UUID) of the registry whose credentials are used to pull a private image. Leave it unset for a public image.
- `secrets` (Map of String) Environment variables whose values come from secrets in the organisation vault. Each entry maps a variable name to a secret.

Read-Only:

- `max_cpu` (Number) The most CPU each replica may use, in millicores, set by Bahriya from the requested CPU. It cannot be set.
- `max_memory` (Number) The most memory each replica may use, in MiB, set by Bahriya from the requested memory. It cannot be set.


<a id="nestedatt--configs"></a>
### Nested Schema for `configs`

Optional:

- `env_files` (Attributes List) Env-files attached to this container. Entries are injected as environment variables via envFrom. (see [below for nested schema](#nestedatt--configs--env_files))
- `json_configs` (Attributes List) JSON configs attached to this container, each mounted as a file. (see [below for nested schema](#nestedatt--configs--json_configs))
- `plain_configs` (Attributes List) Plain-text configs attached to this container, each mounted as a file. (see [below for nested schema](#nestedatt--configs--plain_configs))
- `yaml_configs` (Attributes List) YAML configs attached to this container, each mounted as a file. (see [below for nested schema](#nestedatt--configs--yaml_configs))

<a id="nestedatt--configs--env_files"></a>
### Nested Schema for `configs.env_files`

Required:

- `handle` (String) Handle of the env-file attached to the project.

Optional:

- `injection_method` (String) How the env-file entries are injected into the container. Defaults to `"envFrom"`.


<a id="nestedatt--configs--json_configs"></a>
### Nested Schema for `configs.json_configs`

Required:

- `handle` (String) Handle of the vault/config item attached to the project.
- `mount_path` (String) Absolute mount path inside the container where the item is rendered.


<a id="nestedatt--configs--plain_configs"></a>
### Nested Schema for `configs.plain_configs`

Required:

- `handle` (String) Handle of the vault/config item attached to the project.
- `mount_path` (String) Absolute mount path inside the container where the item is rendered.


<a id="nestedatt--configs--yaml_configs"></a>
### Nested Schema for `configs.yaml_configs`

Required:

- `handle` (String) Handle of the vault/config item attached to the project.
- `mount_path` (String) Absolute mount path inside the container where the item is rendered.



<a id="nestedatt--init_jobs"></a>
### Nested Schema for `init_jobs`

Required:

- `handle` (String) Short identifier for this init job. Unique within the container. Used to address logs and events.
- `image` (String) Container image for the init job. An explicit tag or digest is required — an untagged reference is rejected.

Optional:

- `args` (List of String) Override the image CMD arguments. Trimmed per-element; empty entries dropped.
- `command` (List of String) Override the image ENTRYPOINT — one token per array entry. Use the same conventions as cron/worker: `[/bin/sh, -c]` for shell-style commands, or a direct exec form like `[./migrate]`. Null leaves the image entrypoint untouched.
- `cpu` (Number) CPU for the init job, in millicores (1000 millicores is one vCPU). It cannot exceed the container's requested CPU; init jobs use the container's reserved capacity and are not billed separately. When not set, the init job uses the container's CPU.
- `env` (Map of String) Environment variables for the init job (same shape as the main container).
- `memory` (Number) Memory for the init job, in MiB. It cannot exceed the container's requested memory; init jobs use the container's reserved capacity and are not billed separately. When not set, the init job uses the container's memory.
- `registry` (String) Optional private registry id. Must belong to the same project as the parent container.
- `secrets` (Map of String) Project secrets exposed as env vars for the init job.
- `volumes` (Attributes List) Volumes from the parent container to mount in this init job. Defaults to none — opt-in per volume. (see [below for nested schema](#nestedatt--init_jobs--volumes))

<a id="nestedatt--init_jobs--volumes"></a>
### Nested Schema for `init_jobs.volumes`

Required:

- `handle` (String) Handle of a persistent or ephemeral volume defined on the parent container.
- `mount_path` (String) Mount path inside the init job container. Defaults to the parent container's mount path; can be overridden per init job.



<a id="nestedatt--networking"></a>
### Nested Schema for `networking`

Optional:

- `custom_dns_mode` (String) How traffic is spread across regions on the vanity hostname: none, round-robin or geo. A mode other than none requires two or more regions. Not allowed on cronjob or worker containers.
- `default_hostname` (String) Host alias from which the default hostname in each region is built, such as mamluk-123.helsinki-1.x.on.bahriya.app. Not allowed on cronjob or worker containers.
- `dns_failover` (Boolean) Enable DNS failover for this container. Defaults to `false`. Not allowed on cronjob or worker containers.
- `dns_ttl` (Number) DNS TTL in seconds for vanity (rr/geo) records. Null uses provider default (300s). Region-specific CNAMEs in the containers zone use a separate platform-wide TTL. Not allowed on cronjob or worker containers.
- `domains` (Attributes List) Custom hostnames attached to this container. Up to a configurable maximum (default 5). The first hostname is included; each additional hostname is billed via the extra_custom_hostname SKU. Not allowed on cronjob or worker containers. (see [below for nested schema](#nestedatt--networking--domains))
- `external` (Boolean) Make this container reachable from the internet at its hostnames. When false, only workloads in the same project can reach it. It has no default for HTTP containers, so a hostname is published only when a document says so. Required for http containers. Not allowed on cronjob or worker containers.
- `network_policies` (List of String) Network policies narrowed to this container, referenced by handle. When a policy is listed here it applies to only this container instead of the whole project.
- `vanity_hostname` (String) Platform-managed hostname that spreads traffic across the regions of the container according to the custom DNS mode. It requires two or more regions. Not allowed on cronjob or worker containers.

<a id="nestedatt--networking--domains"></a>
### Nested Schema for `networking.domains`

Required:

- `hostname` (String) Custom hostname (FQDN). Customer is responsible for pointing a CNAME at the container CNAME target. Lowercased and trimmed server-side.

Optional:

- `wwwredirect` (Boolean) Whether www.{hostname} redirects to {hostname}. Valid only when the hostname is an apex domain, such as example.com, and not www.example.com or sub.example.com. Defaults to `false`.



<a id="nestedatt--observability"></a>
### Nested Schema for `observability`

Optional:

- `prometheus_path` (String) Path to Prometheus metrics endpoint. Defaults to /metrics if not specified. Requires prometheus port to be set. Not allowed on cronjob containers.
- `prometheus_port` (Number) Port number exposing Prometheus metrics. Enables automatic scraping when set. Not allowed on cronjob containers.


<a id="nestedatt--proxy_cache"></a>
### Nested Schema for `proxy_cache`

Optional:

- `allow_force_cache_header` (Boolean) Allow clients to send X-Proxy-Cache-Memcached-Force: true to bypass request-method/Cache-Control checks. Defaults to `false`.
- `cache_ttl` (Number) Seconds the plugin serves the cached entry. Defaults to `300`.
- `content_types` (List of String) Response content types eligible for caching (exact match).
- `enabled` (Boolean) Whether responses from the container are cached in front of it, in a managed Memcached cluster. Available on HTTP containers only. Defaults to `false`.
- `honor_cache_control` (Boolean) Honour RFC 7234 Cache-Control directives from upstream. Defaults to `true`.
- `max_item_size_mb` (Number) Memcached -I value. Max size of any single cached item. Must be <= size/3.
- `methods` (List of String) HTTP methods eligible for caching.
- `response_codes` (List of Number) Response status codes eligible for caching.
- `size_mb` (Number) Total size of the response cache, in MB. It must be a multiple of 256. The cache runs on 3 nodes, each with one third of the size rounded down to a whole MB, so a size that is not divisible by 3 is provisioned and billed as the nearest lower multiple of 3: 1024 becomes 1023 (3 × 341). A multiple of 768, such as 768, 1536, 2304 or 3072, is provisioned exactly as given.
- `storage_ttl` (Number) Seconds memcached keeps the entry. Must be >= cache_ttl when set.
- `variant` (String) Pre-rate-limit serves cache hits without consuming a rate-limit token. Post-rate-limit counts every request, hit or miss. Defaults to `"post_ratelimit"`.
- `vary_body_json_fields` (List of String) JSON body fields that contribute to the cache key (scalar values only).
- `vary_headers` (List of String) Request headers that contribute to the cache key.
- `vary_query_params` (List of String) Query params that contribute to the cache key. Null = all params.

Read-Only:

- `memcached` (String) ID of the managed memcached instance backing the proxy cache.


<a id="nestedatt--scaling"></a>
### Nested Schema for `scaling`

Optional:

- `autoscaling` (Boolean) Whether the number of replicas scales automatically between the minimum and maximum replicas. Autoscaling cannot be enabled while persistent volumes are attached. Defaults to `false`. Not allowed on cronjob containers.
- `max_replicas` (Number) Maximum number of replicas when autoscaling is enabled. Setting it enables autoscaling. Not allowed on cronjob containers.
- `replicas` (Number) Number of replicas to run, and the minimum number when autoscaling is enabled. Defaults to `1`.
- `target_cpu` (Number) Average CPU utilisation that autoscaling aims to keep, as a percentage of the requested CPU. It is 100 when not set. Defaults to `100`.
- `target_memory` (Number) Average memory utilisation that autoscaling aims to keep, as a percentage of the requested memory. It is 100 when not set. Defaults to `100`.


<a id="nestedatt--schedule"></a>
### Nested Schema for `schedule`

Optional:

- `active_deadline` (Number) Max wall-time in seconds a single run may take before being killed. Null means no limit.
- `backoff_limit` (Number) Number of retries before a run is considered failed. Defaults to `3`.
- `concurrency` (String) Behaviour when a previous execution is still running at the next fire time. Defaults to `"Forbid"`.
- `failed_history` (Number) How many failed runs are kept in the run history. Older runs are removed automatically. Defaults to `3`.
- `schedule` (String) 5-field cron expression. Required for cron jobs.
- `starting_deadline` (Number) If a scheduled run is missed by more than this many seconds, skip it. Null means catch-up indefinitely.
- `successful_history` (Number) How many successful runs are kept in the run history. Older runs are removed automatically. Defaults to `3`.
- `suspended` (Boolean) Pause scheduling without deleting the cron job. No new runs while suspended. Defaults to `false`.
- `ttl_seconds` (Number) Seconds a finished run is kept, so its logs can be read, before it is removed. Defaults to `3600`.
- `tz` (String) IANA time zone in which the schedule is evaluated, such as Europe/London. Defaults to UTC. The schedule follows this time zone in every region the cron job runs in. Defaults to `"UTC"`.


<a id="nestedatt--security"></a>
### Nested Schema for `security`

Optional:

- `basic_auth` (Attributes List) Up to 10 basic auth credentials (username/password). Passwords are AES-encrypted at rest and never returned in plaintext; send the masked sentinel to preserve an existing password. Not allowed on cronjob or worker containers. (see [below for nested schema](#nestedatt--security--basic_auth))
- `basic_auth_enabled` (Boolean) Whether requests to the container require HTTP basic authentication. Available on HTTP containers only. Defaults to `false`. Not allowed on cronjob or worker containers.
- `ip_allowlist` (Attributes) (see [below for nested schema](#nestedatt--security--ip_allowlist))
- `ip_denylist` (Attributes) (see [below for nested schema](#nestedatt--security--ip_denylist))
- `rate_limiting` (Attributes) (see [below for nested schema](#nestedatt--security--rate_limiting))

<a id="nestedatt--security--basic_auth"></a>
### Nested Schema for `security.basic_auth`

Required:

- `password` (String, Sensitive) Plaintext password on write; returned as the masked sentinel "value-hidden-for-your-own-good" on read. Sending the sentinel preserves the existing stored password.
- `username` (String) User name for HTTP basic authentication: 1 to 128 characters, using letters, digits, full stops, underscores and hyphens. It must be unique across all containers; the console prefixes it with the container handle, such as mywebsite-alice, to keep it unique.


<a id="nestedatt--security--ip_allowlist"></a>
### Nested Schema for `security.ip_allowlist`

Optional:

- `enabled` (Boolean) Enable IP allow-list. Only IPs in the list can access the container. Defaults to `false`. Not allowed on cronjob or worker containers.
- `ips` (List of String) IP addresses and CIDR ranges allowed to reach the container when the allow-list is enabled. Not allowed on cronjob or worker containers.


<a id="nestedatt--security--ip_denylist"></a>
### Nested Schema for `security.ip_denylist`

Optional:

- `enabled` (Boolean) Enable IP deny-list. IPs in the list are blocked. Defaults to `false`. Not allowed on cronjob or worker containers.
- `ips` (List of String) IP addresses and CIDR ranges blocked from reaching the container when the deny-list is enabled. Not allowed on cronjob or worker containers.


<a id="nestedatt--security--rate_limiting"></a>
### Nested Schema for `security.rate_limiting`

Optional:

- `enabled` (Boolean) Whether requests to the container are rate limited per client IP address. Defaults to `false`. Not allowed on cronjob or worker containers.
- `per_hour` (Number) Max requests per hour per IP (requires ratelimitingenabled). Not allowed on cronjob or worker containers.
- `per_minute` (Number) Max requests per minute per IP (requires ratelimitingenabled). Not allowed on cronjob or worker containers.
- `per_second` (Number) Max requests per second per IP (requires ratelimitingenabled).



<a id="nestedatt--storage"></a>
### Nested Schema for `storage`

Optional:

- `ephemeral_storage_gb` (Number) Ephemeral storage in GB per replica. First 3GB included free; each additional GB billed at the ephemeral rate. The maximum is platform-configurable (see GET /console/v1/services/containers). Defaults to `3`.
- `persistent_volumes` (Attributes List) Persistent volumes attached to the container. Each replica gets its own copy of every volume. Containers with persistent volumes cannot use autoscaling. (see [below for nested schema](#nestedatt--storage--persistent_volumes))

<a id="nestedatt--storage--persistent_volumes"></a>
### Nested Schema for `storage.persistent_volumes`

Required:

- `handle` (String) Stable handle for the volume, unique within the container.
- `mount_path` (String) Absolute mount path inside the container, unique within the container.
- `size_gb` (Number) Volume size in GB. Each replica gets its own copy at this size. The per-volume maximum is platform-configurable (see GET /console/v1/services/containers).



<a id="nestedatt--vault"></a>
### Nested Schema for `vault`

Optional:

- `encryption_keys` (Attributes List) Encryption keys attached to this container, mounted as files. Env-var injection is not supported for encryption keys. (see [below for nested schema](#nestedatt--vault--encryption_keys))
- `gpg_keypairs` (Attributes List) GPG keypairs attached to this container, mounted as files. (see [below for nested schema](#nestedatt--vault--gpg_keypairs))
- `ssh_keypairs` (Attributes List) SSH keypairs attached to this container, mounted as files. (see [below for nested schema](#nestedatt--vault--ssh_keypairs))
- `tls_bundles` (Attributes List) TLS bundles mounted into the container. Each entry names a TLS bundle attached to the project, by its handle, and the path at which its files are mounted. (see [below for nested schema](#nestedatt--vault--tls_bundles))
- `x509_certs` (Attributes List) x509 certificates attached to this container, mounted as files. (see [below for nested schema](#nestedatt--vault--x509_certs))

<a id="nestedatt--vault--encryption_keys"></a>
### Nested Schema for `vault.encryption_keys`

Required:

- `handle` (String) Handle of the vault/config item attached to the project.
- `mount_path` (String) Absolute mount path inside the container where the item is rendered.


<a id="nestedatt--vault--gpg_keypairs"></a>
### Nested Schema for `vault.gpg_keypairs`

Required:

- `handle` (String) Handle of the vault/config item attached to the project.
- `mount_path` (String) Absolute mount path inside the container where the item is rendered.


<a id="nestedatt--vault--ssh_keypairs"></a>
### Nested Schema for `vault.ssh_keypairs`

Required:

- `handle` (String) Handle of the vault/config item attached to the project.
- `mount_path` (String) Absolute mount path inside the container where the item is rendered.


<a id="nestedatt--vault--tls_bundles"></a>
### Nested Schema for `vault.tls_bundles`

Required:

- `handle` (String) Handle of the vault/config item attached to the project.
- `mount_path` (String) Absolute mount path inside the container where the item is rendered.


<a id="nestedatt--vault--x509_certs"></a>
### Nested Schema for `vault.x509_certs`

Required:

- `handle` (String) Handle of the vault/config item attached to the project.
- `mount_path` (String) Absolute mount path inside the container where the item is rendered.

## Import

Import a container by its UUID:

```shell
terraform import bahriya_container.api 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```

~> **Note:** The API never returns basic authentication passwords. After importing, set each `security.basic_auth` password in your configuration; the next apply writes them.
