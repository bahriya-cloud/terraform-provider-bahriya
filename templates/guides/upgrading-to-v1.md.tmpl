---
page_title: "Upgrading to version 1 - Bahriya"
subcategory: ""
description: |-
  How to move a configuration from provider 0.x to 1.0: renamed attributes, sections, maps for environment variables, and the automatic state upgrade.
---

# Upgrading to version 1

Version 1.0 gives every resource the same shape as the matching Reis YAML file and the console's YAML view. Attributes have friendly names and are grouped into sections, so a Reis file and the equivalent Terraform resource line up key for key.

This is a breaking change to configurations. Your **state** upgrades automatically; your **configuration** must be rewritten before the first plan with version 1.

## Which resources change

The 18 resources that manage platform items change: `bahriya_project`, `bahriya_container`, `bahriya_path_rule`, `bahriya_memcached`, `bahriya_valkey`, `bahriya_mysql`, `bahriya_network_policy`, `bahriya_registry`, `bahriya_secret` and the vault and config items (`bahriya_tls_bundle`, `bahriya_x509_cert`, `bahriya_gpg_keypair`, `bahriya_ssh_keypair`, `bahriya_encryption_key`, `bahriya_env_file`, `bahriya_yaml_config`, `bahriya_json_config` and `bahriya_plain_config`).

`bahriya_resource_grant` changes three names: `touser` is now `to_user`, `resourcetype` is now `resource_type` and `resourceid` is now `resource_id`. Its state upgrades without replacing the grant.

`bahriya_role` renames `issystem` to `is_system`. Its state also upgrades without replacing the role.

The project attachment resources, `bahriya_billing_details` and the data sources keep their attributes.

## What changes

- **Names.** Attributes use the same friendly name as Reis, in snake case: `mincpu` is now `cpu`, `containerport` is now `port`, `maxversions` is now `max_versions` and `publickey` is now `public_key`.
- **Sections.** Related attributes are grouped into nested objects. A container has `workload`, `scaling`, `networking`, `security`, `storage`, `configs`, `vault`, `schedule`, `observability` and `proxy_cache`; a datastore has sections such as `topology`, `tuning`, `security`, `exposure` and `persistence`. Sections are attributes, so they are written with `=`: `scaling = { replicas = 2 }`.
- **Environment variables and secrets are maps.** A container's environment variables are `workload.env = { NAME = "value" }`, and the secrets it reads are `workload.secrets = { ENV_NAME = "secret-handle" }`. In 0.x both were lists of objects.
- **Regions.** `activeregions` is now `regions`.
- **Registry references.** `workload.registry` takes the registry's ID (`bahriya_registry.ghcr.id`), not its handle.
- **Path rules.** `bahriya_path_rule` names its container in `container`, not `container_id`.
- **Numbers are numbers.** A container's `cpu`, `memory`, `port`, `replicas`, `max_replicas`, `target_cpu`, `target_memory` and `prometheus_port`, and an init job's `cpu` and `memory`, are numbers: write `cpu = 250`, not `cpu = "250"`. (Terraform still accepts the quoted form and converts it.) `cpu` is in millicores, where 1000 is one vCPU, and `memory` in MiB.
- **Timestamps are strings.** `created`, `updated`, `rotated_at` and the other timestamps are RFC 3339 strings in UTC, such as `"2026-11-04T10:00:00Z"`, not objects.
- **Read-only names.** Read-only attributes follow the same naming: `project_name`, `restarted_at`, `managed_by_type`, `current_version`, `effective_quotas.request.cpu`, and the MySQL restore details under `restore`. See the tables below.
- **Defaults.** An attribute you leave out takes its documented default, exactly as in a Reis file. Some defaults depend on the container's `type`: an HTTP container's `port` defaults to 8080 and its `healthcheck` to `/healthz`. In 0.x an omitted attribute kept whatever value the platform last had.
- **`networking.external` is required for HTTP containers.** It has no default: state `true` to serve the container at its hostnames, or `false` to keep it reachable only by the project's other workloads. A plan that leaves it out fails and names the container.
- **Attributes a container's type does not use are refused.** Workers and cron jobs cannot set hostnames, `networking.external`, rate limiting, IP lists, basic authentication or init jobs; cron jobs also cannot set `port`, `healthcheck`, the Prometheus settings, `scaling.autoscaling` or `scaling.max_replicas`. A plan that sets one fails and names the attribute and the type.
- **Errors instead of silent drops.** Setting a read-only or unknown attribute is an error.

Each resource page lists every attribute with its version 1 name.

## Example: a container

Version 0.x:

```terraform
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

  newenvvar = [
    { key = "NODE_ENV", value = "production" },
  ]

  secretsenvvar = [
    { name = "DATABASE_PASSWORD", secret = bahriya_secret.db_password.handle },
  ]

  hostnames = [
    { hostname = "api.example.com" },
  ]

  tlsbundles = [
    { handle = bahriya_tls_bundle.edge.handle, mountpath = "/etc/tls" },
  ]

  networkpolicies = [bahriya_network_policy.web_tier.handle]
}
```

Version 1:

```terraform
resource "bahriya_container" "api" {
  handle  = "api"
  name    = "API Server"
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

    secrets = {
      "DATABASE_PASSWORD" = bahriya_secret.db_password.handle
    }
  }

  scaling = {
    replicas     = 2
    max_replicas = 5
  }

  networking = {
    # Required for an HTTP container. The 0.x container was public, so keep it public.
    external = true
    domains = [
      { hostname = "api.example.com" },
    ]

    network_policies = [bahriya_network_policy.web_tier.handle]
  }

  vault = {
    tls_bundles = [
      { handle = bahriya_tls_bundle.edge.handle, mount_path = "/etc/tls" },
    ]
  }
}
```

## Common renames

### `bahriya_container`

| 0.x | 1.0 |
|-----|-----|
| `image`, `registry`, `containerport`, `healthcheckpath` | `workload.image`, `workload.registry`, `workload.port`, `workload.healthcheck` |
| `mincpu`, `maxcpu`, `minmemory`, `maxmemory` | `workload.cpu`, `workload.max_cpu`, `workload.memory`, `workload.max_memory` |
| `newenvvar`, `secretsenvvar` | `workload.env`, `workload.secrets` (maps) |
| `autoscalingenabled`, `autoscalingminreplicas`, `autoscalingmaxreplicas` | `scaling.autoscaling`, `scaling.replicas`, `scaling.max_replicas` |
| `autoscalingtargetcpu`, `autoscalingtargetmemory` | `scaling.target_cpu`, `scaling.target_memory` |
| `hostnames`, `externalnetworkingenabled`, `networkpolicies` | `networking.domains`, `networking.external`, `networking.network_policies` |
| `ratelimitingenabled`, `ratelimitingrequestsperminute` | `security.rate_limiting.enabled`, `security.rate_limiting.per_minute` |
| `ipwhitelistenabled`, `ipwhitelist` | `security.ip_allowlist.enabled`, `security.ip_allowlist.ips` |
| `ipblacklistenabled`, `ipblacklist` | `security.ip_denylist.enabled`, `security.ip_denylist.ips` |
| `basicauthenabled`, `basicauthcredentials` | `security.basic_auth_enabled`, `security.basic_auth` |
| `schedule`, `timezone`, `concurrencypolicy` | `schedule.schedule`, `schedule.tz`, `schedule.concurrency` |
| `tlsbundles`, `sshkeypairs`, `envfiles`, `yamlconfigs` | `vault.tls_bundles`, `vault.ssh_keypairs`, `configs.env_files`, `configs.yaml_configs` |

### `bahriya_mysql`

| 0.x | 1.0 |
|-----|-----|
| `cpumillicores`, `memorymb`, `storagegb`, `replicas` | `cpu`, `memory`, `persistence.storage`, `topology.replicas` |
| `tlsenabled`, `tlsbundle`, `tlsvalidityyears` | `security.tls`, `security.tls_bundle`, `security.tls_validity_years` |
| `encryptionenabled`, `volumeencryptionenabled` | `security.encryption`, `security.volume_encryption` |
| `externalenabled`, `externalport`, `allowedips`, `hostnames` | `exposure.external`, `exposure.external_port`, `exposure.allowed_ips`, `exposure.hostnames` |
| `clientenabled`, `clientttlhours` | `client.phpmyadmin`, `client.phpmyadmin_ttl` |
| `backupschedule`, `config`, `networkpolicies` | `backup.backup_schedule`, `tuning.config`, `networking.network_policies` |

### `bahriya_valkey`

| 0.x | 1.0 |
|-----|-----|
| `memorymb`, `storagegb`, `size`, `shards` | `memory`, `persistence.storage`, `topology.size`, `topology.shards` |
| `authenabled`, `password` | `security.auth`, `security.password` |
| `tlsenabled`, `tlsbundle`, `tlsvalidityyears` | `security.tls`, `security.tls_bundle`, `security.tls_validity_years` |
| `externalenabled`, `allowedips`, `hostnames` | `exposure.external`, `exposure.allowed_ips`, `exposure.hostnames` |
| `backupenabled`, `backupschedule` | `backup.backup`, `backup.backup_schedule` |
| `maxmemorypolicy`, `config`, `networkpolicies` | `tuning.maxmemory_policy`, `tuning.config`, `networking.network_policies` |

### `bahriya_memcached`

| 0.x | 1.0 |
|-----|-----|
| `nodes` | `topology.nodes` |
| `memorymb`, `maxconnections`, `threads`, `maxitemsizemb` | `tuning.memory`, `tuning.max_connections`, `tuning.threads`, `tuning.max_item_size_mb` |
| `networkpolicies` | `network_policies` |

### `bahriya_network_policy`

| 0.x | 1.0 |
|-----|-----|
| `ingresspeers`, `egresscidrs`, `egressfqdns`, `l7enabled` | `ingress_peers`, `egress_cidrs`, `egress_fqdns`, `enable_l7` |

### Vault and config items

| 0.x | 1.0 |
|-----|-----|
| `publickey`, `privatekey` | `public_key`, `private_key` |
| `maxversions`, `versioncount`, `rotatedat` | `max_versions`, `version_count`, `rotated_at` |
| `keybits`, `keyid`, `notbefore`, `notafter` | `key_bits`, `key_id`, `not_before`, `expires` |
| `managedbyresourceid`, `managedbyresourcetype` | `managed_by`, `managed_by_type` |
| `currentversion`, `currentversionid` | `current_version`, `current_version_id` |

### Read-only attributes elsewhere

| Resource | 0.x | 1.0 |
|----------|-----|-----|
| `bahriya_container`, `bahriya_memcached`, `bahriya_valkey`, `bahriya_mysql` | `projectname` | `project_name` |
| `bahriya_container` | `restartedat` | `restarted_at` |
| `bahriya_memcached` | `managedbycontainer`, `managedbycontainername` | `managed_by_container`, `managed_by_container_name` |
| `bahriya_project` | `effectivequotarequestcpu`, `effectivequotarequestmemory` | `effective_quotas.request.cpu`, `effective_quotas.request.memory` |
| `bahriya_project` | `effectivequotalimitcpu`, `effectivequotalimitmemory` | `effective_quotas.limit.cpu`, `effective_quotas.limit.memory` |
| `bahriya_mysql` | `restoreof`, `restoreofhandle`, `restorekind`, `restoretargetat`, `restoredatabase`, `restorestatus`, `restorefinishedat`, `restoreerror`, `restoretargets` | `restore.source`, `restore.source_handle`, `restore.kind`, `restore.target_at`, `restore.database`, `restore.status`, `restore.finished_at`, `restore.error`, `restore.targets` |
| `bahriya_role` | `issystem` | `is_system` |

Update any output or reference that reads one of these attributes.

## Upgrade steps

1. Commit your configuration and back up your state (`terraform state pull > backup.tfstate`).
2. Change the version constraint to `~> 1.0` and run `terraform init -upgrade`.
3. Rewrite each changed resource in the version 1 shape, using the tables above and each resource's page.
4. State `networking.external` on every HTTP container: `true` to keep its hostnames, `false` to remove them. A container that was public under 0.x is public now, so `true` keeps it as it is.
5. Remove any attribute a container's type does not use, such as rate limiting on a worker. The plan names each one.
6. Run `terraform plan`. The state upgrades on this first plan. If the rewrite matches your running resources, the plan shows **no changes**. A change it does show comes from the configuration: usually an attribute that was left out and now takes its default, or a rename that was missed.
7. Apply once the plan shows only the changes you intend.

## State from older releases

State written by provider 0.3.0 or later upgrades directly. For older state, apply once with provider 0.4.2, run `terraform apply -refresh-only`, and then switch to version 1. A state that cannot be upgraded is refused with an error that names the resource; nothing in it is dropped.

Values the API never returns, such as secret values, keys and passwords, are carried over from your state, so the upgrade does not rotate them.
