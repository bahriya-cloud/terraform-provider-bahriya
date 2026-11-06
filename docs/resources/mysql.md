---
page_title: "bahriya_mysql Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya MySQL instance — a managed MySQL database with per-database users, backups and point-in-time recovery.
---

# bahriya_mysql (Resource)

Manages a Bahriya MySQL instance. An instance runs one or more logical databases, each with its own user, in every region you choose. Backups are always on.

Three choices shape an instance:

- `tier` — `single` (one node; after a failure it restarts from its own disk), `replicated` (a primary plus one or two asynchronous read replicas; writes go to the primary and there is no automatic failover) or `ha` (Group Replication across three or five members, with automatic failover). Immutable after creation — changing it forces recreation.
- The plan — `cpumillicores` and `memorymb` per node, chosen together as a pair from the published plan matrix. Every node runs the plan, so a `replicated` instance with one read replica runs two nodes.
- `encryptionenabled` — MySQL encryption at rest (every table, the redo and undo logs and the binary logs), decided at creation. `volumeencryptionenabled` adds a separately billed layer that encrypts every volume. Both keys are platform-generated and platform-managed. Changing either forces recreation; there is no online migration between encrypted and unencrypted storage.

Creating a MySQL instance is asynchronous — the provider waits up to 20 minutes for the instance to reach `running` status. Destroying waits up to 20 minutes for `terminated` status. If a create does not settle in time, the instance is saved to state with its current status and a warning; run `terraform apply` again to re-check.

Passwords are never part of the resource or its state. The platform generates the root password and one password per database user and keeps them as platform-managed secrets in your vault (`rootsecret`, `databasesecrets`), billed like any secret; reveal or rotate them from the console or with `reis mysql:credentials`. With encryption, the volume passphrase and an escrow of the encryption keys are platform-managed encryption keys in your vault (`lukskey`, `keyringkey`). Rotate the encryption master key from the console or with `reis mysql:rotate-key`.

## Example Usage

### Single Node

```terraform
resource "bahriya_mysql" "orders" {
  handle        = "orders-db"
  name          = "Orders DB"
  tier          = "single"
  cpumillicores = 1000
  memorymb      = 2048
  storagegb     = 20
  activeregions = ["falkenstein-1"]
  project       = bahriya_project.web.id

  databases = [
    { name = "orders", username = "orders-app" },
  ]
}
```

### Replicated with Several Databases and Tuning

```terraform
resource "bahriya_mysql" "shop" {
  handle        = "shop-db"
  name          = "Shop DB"
  tier          = "replicated"
  replicas      = 2
  cpumillicores = 2000
  memorymb      = 8192
  storagegb     = 100
  activeregions = ["falkenstein-1"]
  project       = bahriya_project.web.id

  databases = [
    { name = "catalog", username = "catalog-app" },
    { name = "reporting", username = "reporting-ro" },
  ]

  config = {
    "max_connections" = "500"
    "slow_query_log"  = "true"
    "long_query_time" = "2"
  }

  backupschedule = "0 3 * * *"
}
```

Adding or removing an entry in `databases` on a live instance converges it: a new entry creates the database and its user; **a removed entry drops that database and its user, with all their data, in every region**. Schemas you created yourself over SQL are never touched. Review the plan before applying a change that shortens the list.

### Highly Available, Encrypted, with External Access

External access requires TLS and a non-empty CIDR allowlist — the API rejects any state where `externalenabled` is true and either is missing. The instance is reachable at `<defaulthostname>.<region>.mysql.on.bahriya.app` on the port in `externalport`, which is allocated when external access is first enabled and is the same in every region.

```terraform
resource "bahriya_mysql" "ledger" {
  handle            = "ledger-db"
  name              = "Ledger DB"
  tier              = "ha"
  replicas          = 3
  cpumillicores     = 4000
  memorymb          = 16384
  storagegb         = 250
  encryptionenabled = true
  activeregions     = ["falkenstein-1"]
  project           = bahriya_project.web.id

  databases = [
    { name = "ledger", username = "ledger-app" },
  ]

  tlsenabled       = true
  tlsvalidityyears = 1
  externalenabled  = true
  allowedips       = ["198.51.100.0/24"]

  # Temporary browser access, switched off automatically after 8 hours.
  clientenabled  = true
  clientttlhours = 8

  hostnames = [
    { hostname = "db.example.com" },
  ]
}

output "ledger_port" {
  value = bahriya_mysql.ledger.externalport
}
```

With `tlsenabled = true` and no `tlsbundle`, the platform issues and manages a TLS bundle in your vault. Its server certificate lasts `tlsvalidityyears` (1, 3, 5 or 10; 3 by default) and is renewed automatically 30 days before it expires. Every renewal, and every reissue when you add a hostname or region, is signed by the same CA, so a CA certificate your clients saved keeps working. To serve your own bundle instead, set `tlsbundle` to its handle — it must cover every hostname the instance answers to, and the API returns a 400 listing any missing names. Clients connect with `ssl-mode=VERIFY_IDENTITY`, passing the bundle's CA certificate (shown in the vault) with `--ssl-ca`.

`storagegb` is grow-only in self-service. Shrinking requires a new instance restored from a backup.

## Schema

Nested items (`databases`, `hostnames`) are Attributes Lists — assign them with attribute syntax (`databases = [{ name = "...", username = "..." }]`), not block syntax.

### Required

- `handle` (String) - Unique lowercase handle. Immutable — changing this forces recreation. Handles are **not released** on delete.
- `name` (String) - Display name.
- `tier` (String) - Product tier. single = one node, restarts from disk. replicated = a primary plus asynchronous read replicas. ha = Group Replication with automatic failover. Immutable after creation — changing this forces recreation.
- `cpumillicores` (Number) - CPU per node in millicores. Validated together with memorymb as a pair against the published plan matrix.
- `memorymb` (Number) - Memory per node in MB. Validated together with cpumillicores as a pair against the published plan matrix.
- `storagegb` (Number) - Persistent data storage per node in GB. Grow-only in self-service. The binlog volume backing point-in-time recovery is platform-managed and not counted here.
- `databases` (Attributes List) - Logical databases inside the instance, each with a dedicated user. At least one is required. Add or remove entries on a live instance and the platform converges it; schemas created by hand over SQL are never touched. Passwords are platform-generated — reveal or rotate them via the credentials endpoint. (see [below for nested schema](#nestedatt--databases))
- `activeregions` (List of String) - Regions this instance deploys into. At least one is required. Each region runs an independent copy with the same configuration and its own data.

### Optional

- `project` (String) - Project UUID. Changing this forces recreation.
- `replicas` (Number) - replicated tier: read replicas, 1 or 2 (default 1). ha tier: group members including the primary, 3 or 5 (default 3). Not applicable to single.
- `config` (Map of String) - Allowlisted mysqld configuration keys. Unknown keys are rejected naming the key. Supported keys: `max_connections`, `wait_timeout`, `interactive_timeout`, `max_allowed_packet`, `group_concat_max_len`, `innodb_flush_log_at_trx_commit`, `character_set_server`, `collation_server`, `sql_mode`, `slow_query_log`, `long_query_time`. Values are given as strings; numbers and `true`/`false` are normalised server-side.
- `tlsenabled` (Boolean) - Serve MySQL-native TLS (clients connect with ssl-mode VERIFY_IDENTITY). Required while external access is enabled.
- `tlsbundle` (String) - Handle of a customer TLS bundle to serve. Omitted with TLS enabled, the platform issues and manages a bundle in your vault. A supplied bundle must cover every hostname the instance answers to; a 400 lists any missing names.
- `tlsvalidityyears` (Number) - Lifetime in years of the server certificate the platform issues when TLS is on with no customer bundle: 1, 3, 5 or 10. Renewed automatically 30 days before expiry, and reissued when changed, always signed by the same 30-year CA, so a saved CA certificate keeps verifying the server. Ignored for a customer bundle.
- `encryptionenabled` (Boolean) - Encrypt data at rest with MySQL InnoDB encryption: every table, the redo and undo logs and the binary logs, under platform-generated and platform-managed keys (escrowed in your vault as keyringkey). Decided at creation and immutable thereafter - there is no online migration between the two states. Changing this forces recreation.
- `volumeencryptionenabled` (Boolean) - Add-on: encrypt every volume of the instance (data, binary logs, temporary files) at the storage layer, on top of encryptionenabled. The passphrase is platform-generated and platform-managed, kept in your vault as lukskey. Billed separately. Decided at creation and immutable thereafter. Changing this forces recreation.
- `defaulthostname` (String) - Server-generated default hostname (use the hostname generator endpoint). Assigned on create, immutable thereafter. Only required up front when supplying a customer `tlsbundle`, so SAN coverage can be validated.
- `externalenabled` (Boolean) - Expose the instance at the platform edge on an allocated port. Requires TLS and a non-empty IP allowlist.
- `allowedips` (List of String) - CIDR allowlist for external access. Required non-empty while external is enabled.
- `hostnames` (Attributes List) - Custom hostnames. A custom hostname is a vanity CNAME to the per-region default hostname - connections still use the allocated port. (see [below for nested schema](#nestedatt--hostnames))
- `clientenabled` (Boolean) - Run the phpMyAdmin web client at pma-<defaulthostname>.<region>... over HTTPS, restricted to the IP allowlist. Intended for temporary access: it is switched off automatically when its time limit is reached.
- `clientttlhours` (Number) - The phpMyAdmin client's time limit in hours. The client is switched off automatically when it is reached. Defaults to 24 when the client is enabled without one.
- `backupschedule` (String) - Dump cron schedule. Backups are always on for MySQL - only the schedule is tunable. Omitted, the platform default applies.
- `networkpolicies` (List of String) - Network policies narrowed to this MySQL instance, referenced by handle.

### Read-Only

- `id` (String) - MySQL instance UUID.
- `status` (String) - Current instance status.
- `nodes` (Number) - Derived node count per region for the current tier and replicas.
- `externalport` (Number) - The edge port allocated to this instance - the same port in every region. Assigned when external access is first enabled; never customer-chosen.
- `organisation` (String) - Organisation UUID.
- `projectname` (String) - Project display name.
- `rootsecret` (String) - Handle of the platform-managed vault secret holding the root password.
- `databasesecrets` (List of String) - Handles of the platform-managed vault secrets holding each database user's password (<handle>-mysql-<database>).
- `lukskey` (String) - Handle of the platform-managed vault encryption key holding the volume encryption passphrase. Null without volume encryption.
- `keyringkey` (String) - Handle of the platform-managed vault encryption key escrowing the instance's InnoDB keyrings. Null without encryption, and until the first escrow after the instance is running.
- `keyrotationstatus` (String) - Outcome of the latest master-key rotation (rotate-key). Null when none was requested.
- `keyrotationrequestedat` (String) - When the latest master-key rotation was requested.
- `keyrotationfinishedat` (String) - When the latest master-key rotation finished.

<a id="nestedatt--databases"></a>

#### Nested Schema for `databases` (Attributes List)

- `name` (String, Required) - Schema name.
- `username` (String, Required) - The dedicated user granted on this schema.

<a id="nestedatt--hostnames"></a>

#### Nested Schema for `hostnames` (Attributes List)

- `hostname` (String, Required) - Custom hostname (FQDN). Point a CNAME at the instance's per-region hostname.

## Import

MySQL instances can be imported using their UUID:

```bash
terraform import bahriya_mysql.orders 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
