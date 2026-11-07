---
page_title: "bahriya_mysql Resource - Bahriya"
subcategory: ""
description: |-
  A managed MySQL instance: one or more databases, each with its own user, with backups and point-in-time recovery.
---

# bahriya_mysql (Resource)

A managed MySQL instance runs one or more databases, each with its own user, in every region of `regions`. Backups are always on.

Three choices shape an instance:

- `tier`: `single` (one node, which restarts from its own disk after a failure), `replicated` (a primary and one or two asynchronous read replicas; writes go to the primary and there is no automatic failover) or `ha` (Group Replication across three or five members, with automatic failover). Changing it replaces the instance.
- The plan: `cpu` and `memory` per node, chosen together as one of the published plans. Every node runs the plan, so a `replicated` instance with one read replica runs two nodes.
- Encryption: `security.encryption` turns on MySQL encryption at rest for every table, the redo and undo logs and the binary logs. `security.volume_encryption` adds a separately billed layer that encrypts every volume. Bahriya generates and manages the keys for both. Both are decided at creation; changing either replaces the instance, because there is no online move between encrypted and unencrypted storage.

Creating, changing and deleting an instance is asynchronous. The provider waits up to 20 minutes for the instance to reach `running`, or `terminated` on delete. If a create does not settle in time, the instance is saved to state with its current status and a warning; run `terraform apply` again to check it.

Passwords are never part of the resource or its state. Bahriya generates the root password and one password per database user, and keeps them in your vault as managed secrets (`root_secret` and `database_secrets`), billed like any other secret. Reveal or rotate them in the console or with `reis mysql:credentials`. With encryption on, the volume passphrase and a copy of the encryption keys are kept in your vault as managed encryption keys (`luks_key` and `keyring_key`). Rotate the encryption master key in the console or with `reis mysql:rotate-key`.

## Example Usage

### A single node

```terraform
resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_mysql" "orders" {
  handle  = "orders-db"
  name    = "Orders DB"
  tier    = "single"
  cpu     = 1000
  memory  = 2048
  regions = ["falkenstein-1"]
  project = bahriya_project.web.id

  persistence = {
    storage = 20
  }

  databases = [
    { name = "orders", username = "orders-app" },
  ]
}
```

### Replicated, with several databases and tuning

```terraform
resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_mysql" "shop" {
  handle  = "shop-db"
  name    = "Shop DB"
  tier    = "replicated"
  cpu     = 2000
  memory  = 8192
  regions = ["falkenstein-1"]
  project = bahriya_project.web.id

  topology = {
    replicas = 2
  }

  persistence = {
    storage = 100
  }

  databases = [
    { name = "catalog", username = "catalog-app" },
    { name = "reporting", username = "reporting-ro" },
  ]

  # Free-form server settings. Values are strings.
  tuning = {
    config = {
      "max_connections" = "500"
      "slow_query_log"  = "true"
      "long_query_time" = "2"
    }
  }

  backup = {
    backup_schedule = "0 3 * * *"
  }
}
```

Adding or removing an entry in `databases` on a running instance takes effect in place. A new entry creates the database and its user. **A removed entry drops that database and its user, with all their data, in every region.** Databases you created yourself over SQL are never touched. Review the plan before applying a change that shortens the list.

### Highly available and encrypted, with external access

External access needs TLS and at least one address range in `exposure.allowed_ips`; the API refuses `exposure.external = true` without both. The instance is then reachable at `<default_hostname>.<region>.mysql.on.bahriya.app` on the port in `exposure.external_port`. The port is assigned when external access is first turned on, and is the same in every region.

```terraform
resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_mysql" "ledger" {
  handle  = "ledger-db"
  name    = "Ledger DB"
  tier    = "ha"
  cpu     = 4000
  memory  = 16384
  regions = ["falkenstein-1"]
  project = bahriya_project.web.id

  topology = {
    replicas = 3
  }

  persistence = {
    storage = 250
  }

  databases = [
    { name = "ledger", username = "ledger-app" },
  ]

  security = {
    encryption         = true
    tls                = true
    tls_validity_years = 1
  }

  exposure = {
    external    = true
    allowed_ips = ["198.51.100.0/24"]
    hostnames = [
      { hostname = "db.example.com" },
    ]
  }

  # Temporary browser access, switched off automatically after 8 hours.
  client = {
    phpmyadmin     = true
    phpmyadmin_ttl = 8
  }
}

output "ledger_port" {
  value = bahriya_mysql.ledger.exposure.external_port
}
```

With `security.tls = true` and no `security.tls_bundle`, Bahriya issues and manages a TLS bundle in your vault. Its server certificate lasts `security.tls_validity_years` (1, 3, 5 or 10; 3 by default) and is renewed automatically 30 days before it expires. Every renewal, and every reissue when you add a hostname or region, is signed by the same CA, so a CA certificate your clients saved keeps working. To serve your own bundle instead, set `security.tls_bundle` to its handle. It must cover every hostname the instance answers to; the API refuses it with a 400 that lists any missing names. Clients connect with `ssl-mode=VERIFY_IDENTITY`, passing the bundle's CA certificate with `--ssl-ca`. The `bahriya_tls_bundle` data source reads that certificate.

`persistence.storage` can only grow. To shrink an instance, restore a backup into a new, smaller one.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `cpu` (Number) CPU per node in millicores. Validated together with memorymb as a pair against the published plan matrix.
- `databases` (Attributes List) Logical databases inside the instance, each with a dedicated user. At least one is required. Add or remove entries on a live instance and the platform converges it; schemas created by hand over SQL are never touched. Passwords are platform-generated — reveal or rotate them via the credentials endpoint. (see [below for nested schema](#nestedatt--databases))
- `handle` (String) Unique handle of the MySQL instance: lowercase letters, digits and hyphens.
- `memory` (Number) Memory per node in MB. Validated together with cpumillicores as a pair against the published plan matrix.
- `name` (String) Display name of the MySQL instance, up to 100 characters.
- `persistence` (Attributes) (see [below for nested schema](#nestedatt--persistence))
- `regions` (List of String) Regions this instance deploys into. At least one is required. Each region runs an independent copy with the same configuration and its own data.
- `tier` (String) Product tier. single = one node, restarts from disk. replicated = a primary plus asynchronous read replicas. ha = Group Replication with automatic failover. Immutable after creation.

### Optional

- `backup` (Attributes) (see [below for nested schema](#nestedatt--backup))
- `client` (Attributes) (see [below for nested schema](#nestedatt--client))
- `default_hostname` (String) Server-generated default hostname (use the hostname generator endpoint). Assigned on create, immutable thereafter.
- `exposure` (Attributes) (see [below for nested schema](#nestedatt--exposure))
- `networking` (Attributes) (see [below for nested schema](#nestedatt--networking))
- `project` (String) Identifier (UUID) of the project the MySQL instance belongs to. Set when the instance is created and cannot be changed.
- `security` (Attributes) (see [below for nested schema](#nestedatt--security))
- `topology` (Attributes) (see [below for nested schema](#nestedatt--topology))
- `tuning` (Attributes) (see [below for nested schema](#nestedatt--tuning))

### Read-Only

- `created` (Attributes) When the MySQL instance was created (UTC). (see [below for nested schema](#nestedatt--created))
- `database_secrets` (List of String) Handles of the platform-managed vault secrets holding each database user's password (<handle>-mysql-<database>).
- `id` (String) Unique identifier (UUID) of the MySQL instance, assigned by the platform.
- `key_rotation_finished_at` (String) When the latest master-key rotation finished.
- `key_rotation_requested_at` (String) When the latest master-key rotation was requested.
- `key_rotation_status` (String) Outcome of the latest master-key rotation (rotate-key). Null when none was requested.
- `keyring_key` (String) Handle of the platform-managed vault encryption key escrowing the instance's InnoDB keyrings. Null without encryption, and until the first escrow after the instance is running.
- `luks_key` (String) Handle of the platform-managed vault encryption key holding the volume encryption passphrase. Null without volume encryption.
- `nodes` (Number) Derived node count per region for the current tier and replicas.
- `organisation` (String) Identifier (UUID) of the organisation that owns the MySQL instance.
- `projectname` (String) Name of the project the MySQL instance belongs to.
- `restoredatabase` (String) Set on a restore target that restored one database only.
- `restoreerror` (String) Set on a failed restore target: why it failed.
- `restorefinishedat` (String) Set on a restore target: when the restore completed or failed.
- `restorekind` (String) Set on a restore target: staging (a disposable copy, no backups) or instance (kept, with backups).
- `restoreof` (String) Set on a restore target: the id of the instance it was restored from.
- `restoreofhandle` (String) Set on a restore target: the handle of the instance it was restored from.
- `restorestatus` (String) Set on a restore target: whether its data is ready. Use the instance only once this is completed.
- `restoretargetat` (String) Set on a restore target: the UTC instant its data was restored to.
- `restoretargets` (Attributes List) On a source instance: its restore targets that are not terminated. (see [below for nested schema](#nestedatt--restoretargets))
- `root_secret` (String) Handle of the platform-managed vault secret holding the root password.
- `status` (String) Lifecycle state of the MySQL instance: provisioning, running, terminating, terminated, suspended or error.
- `updated` (Attributes) When the MySQL instance was last changed (UTC). (see [below for nested schema](#nestedatt--updated))

<a id="nestedatt--databases"></a>
### Nested Schema for `databases`

Required:

- `name` (String) Schema name.
- `username` (String) The dedicated user granted on this schema.


<a id="nestedatt--persistence"></a>
### Nested Schema for `persistence`

Required:

- `storage` (Number) Persistent data storage per node in GB. Grow-only in self-service. The binlog volume backing point-in-time recovery is platform-managed and not counted here.


<a id="nestedatt--backup"></a>
### Nested Schema for `backup`

Optional:

- `backup_schedule` (String) Dump cron schedule. Backups are always on for MySQL - only the schedule is tunable. Omitted, the platform default applies.


<a id="nestedatt--client"></a>
### Nested Schema for `client`

Optional:

- `phpmyadmin` (Boolean) Run the phpMyAdmin web client at pma-<defaulthostname>.<region>... over HTTPS, restricted to the IP allowlist. Intended for temporary access: it is switched off automatically when its time limit is reached. Defaults to `false`.
- `phpmyadmin_ttl` (Number) The phpMyAdmin client's time limit in hours. The client is switched off automatically when it is reached. Defaults to 24 when the client is enabled without one.

Read-Only:

- `phpmyadmin_expires_at` (String) When the phpMyAdmin client's time limit runs out (UTC). Null while the client is off. Set again each time the client is switched on or its time limit changes.


<a id="nestedatt--exposure"></a>
### Nested Schema for `exposure`

Optional:

- `allowed_ips` (List of String) CIDR allowlist for external access. Required non-empty while external is enabled.
- `external` (Boolean) Expose the instance at the platform edge on an allocated port. Requires TLS and a non-empty IP allowlist. Defaults to `false`.
- `hostnames` (Attributes List) Custom hostnames. A custom hostname is a vanity CNAME to the per-region default hostname - connections still use the allocated port. (see [below for nested schema](#nestedatt--exposure--hostnames))

Read-Only:

- `external_port` (Number) The edge port allocated to this instance - the same port in every region. Assigned when external access is first enabled; never customer-chosen.

<a id="nestedatt--exposure--hostnames"></a>
### Nested Schema for `exposure.hostnames`

Required:

- `hostname` (String) A custom hostname (fully qualified domain name) for the instance.



<a id="nestedatt--networking"></a>
### Nested Schema for `networking`

Optional:

- `network_policies` (List of String) Network policies narrowed to this MySQL instance, referenced by handle.


<a id="nestedatt--security"></a>
### Nested Schema for `security`

Optional:

- `encryption` (Boolean) Encrypt data at rest with MySQL InnoDB encryption: every table, the redo and undo logs and the binary logs, under platform-generated and platform-managed keys (escrowed in your vault as keyringkey). Decided at creation and immutable thereafter - there is no online migration between the two states. Defaults to `false`.
- `tls` (Boolean) Serve MySQL-native TLS (clients connect with ssl-mode VERIFY_IDENTITY). Required while external access is enabled. Defaults to `false`.
- `tls_bundle` (String) Handle of a customer TLS bundle to serve. Omitted with TLS enabled, the platform issues and manages a bundle in your vault. A supplied bundle must cover every hostname the instance answers to; a 400 lists any missing names.
- `tls_validity_years` (Number) Lifetime in years of the server certificate the platform issues when TLS is on with no customer bundle: 1, 3, 5 or 10. Renewed automatically 30 days before expiry, and reissued when changed, always signed by the same 30-year CA, so a saved CA certificate keeps verifying the server. Ignored for a customer bundle. Defaults to `3`.
- `volume_encryption` (Boolean) Add-on: encrypt every volume of the instance (data, binary logs, temporary files) at the storage layer, on top of encryptionenabled. The passphrase is platform-generated and platform-managed, kept in your vault as lukskey. Billed separately. Decided at creation and immutable thereafter. Defaults to `false`.


<a id="nestedatt--topology"></a>
### Nested Schema for `topology`

Optional:

- `replicas` (Number) replicated tier: read replicas, 1 or 2 (default 1). ha tier: group members including the primary, 3 or 5 (default 3). Not applicable to single.


<a id="nestedatt--tuning"></a>
### Nested Schema for `tuning`

Optional:

- `config` (Map of String) Allowlisted mysqld configuration keys. Unknown keys are rejected naming the key.


<a id="nestedatt--created"></a>
### Nested Schema for `created`

Read-Only:

- `date` (String)
- `timezone` (String)
- `timezonetype` (Number)


<a id="nestedatt--restoretargets"></a>
### Nested Schema for `restoretargets`

Read-Only:

- `handle` (String) Handle of the restore target instance.
- `id` (String) Identifier (UUID) of the restore target instance.
- `instancestatus` (String) Lifecycle state of the restore target instance: provisioning, running, terminating, terminated, suspended or error.
- `kind` (String) Kind of restore target: staging (a disposable copy, without backups) or instance (kept, with backups).
- `status` (String) Progress of the restore: pending, restoring, completed or failed.
- `targetat` (String) The UTC instant the restore target's data was restored to.


<a id="nestedatt--updated"></a>
### Nested Schema for `updated`

Read-Only:

- `date` (String)
- `timezone` (String)
- `timezonetype` (Number)

## Import

Import a MySQL instance by its UUID:

```shell
terraform import bahriya_mysql.orders 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
