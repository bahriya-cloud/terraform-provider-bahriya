# terraform-provider-bahriya

The official Terraform provider for [Bahriya](https://bahriya.cloud) — manage every Bahriya resource declaratively.

> **Status:** pre-release. Not yet published to the Terraform registry. Track progress on Nextcloud Deck card #928.

---

## Table of contents

- [Installation](#installation)
- [Quickstart](#quickstart)
- [Provider configuration](#provider-configuration)
- [Authentication](#authentication)
- [Resources](#resources)
- [Data sources](#data-sources)
- [Important behaviours](#important-behaviours)
- [Examples](#examples)
- [Building from source](#building-from-source)
- [Local development](#local-development)
- [Architecture](#architecture)
- [Repository layout](#repository-layout)
- [Testing](#testing)

---

## Installation

### Once published (target state)

```hcl
terraform {
  required_providers {
    bahriya = {
      source  = "bahriya-cloud/bahriya"
      version = "~> 0.1"
    }
  }
}
```

### Currently — from source

See [Building from source](#building-from-source) and [Local development](#local-development) below.

## Quickstart

```hcl
terraform {
  required_providers {
    bahriya = { source = "bahriya-cloud/bahriya" }
  }
}

provider "bahriya" {}

data "bahriya_regions" "active" {
  status_filter = "active"
}

resource "bahriya_project" "web" {
  handle  = "web-prod"
  name    = "Web Production"
  regions = [data.bahriya_regions.active.regions[0].id]
}

resource "bahriya_registry" "ghcr" {
  handle   = "ghcr"
  name     = "GitHub Container Registry"
  server   = "ghcr.io"
  username = var.ghcr_username
  password = var.ghcr_token
}

resource "bahriya_secret" "db_password" {
  handle = "db-password"
  name   = "Database Password"
  value  = var.db_password
}

resource "bahriya_container" "api" {
  handle    = "api"
  name      = "API Server"
  image     = "ghcr.io/myorg/api:1.2.3"

  # Required for HTTP containers — deploy will fail without these
  containerport   = "3000"
  healthcheckpath = "/healthz"

  mincpu    = "250"
  minmemory = "256"

  autoscalingminreplicas = "2"
  activeregions          = [data.bahriya_regions.active.regions[0].id]
  project                = bahriya_project.web.id
  registry               = bahriya_registry.ghcr.handle

  secretsenvvar {
    secret = bahriya_secret.db_password.handle
    name   = "DATABASE_PASSWORD"
  }

  hostnames {
    hostname = "api.example.com"
  }
}
```

## Provider configuration

| Attribute         | Required | Env var                    | Description                                                                |
| ----------------- | -------- | -------------------------- | -------------------------------------------------------------------------- |
| `token`           | yes      | `BAHRIYA_TOKEN`            | Personal access token (PAT). Issued in the console under your profile.     |
| `organisation_id` | yes      | `BAHRIYA_ORGANISATION_ID`  | The UUID of the org these resources belong to.                             |
| `base_url`        | no       | `BAHRIYA_API_URL`          | API root. Defaults to `https://api.bahriya.cloud/console/v1`.              |

Attributes on the `provider` block override env vars. For multi-org setups, use `alias`:

```hcl
provider "bahriya" {
  alias           = "prod"
  organisation_id = var.prod_org_id
}

provider "bahriya" {
  alias           = "staging"
  organisation_id = var.staging_org_id
}
```

## Authentication

The provider uses Bahriya **personal access tokens** (PATs):

1. Sign in to the console at <https://app.bahriya.cloud>.
2. Open your profile, then **Tokens**.
3. Click **Create token**, copy the `pat_...` value.

```bash
export BAHRIYA_TOKEN="pat_..."
export BAHRIYA_ORGANISATION_ID="65cc42f1-..."
```

## Resources

| Resource                  | Description                                                               | E2E Verified |
| ------------------------- | ------------------------------------------------------------------------- | ------------ |
| `bahriya_project`         | Logical grouping of containers, secrets, registries; carries quota.       | Yes          |
| `bahriya_container`       | A workload: HTTP server, worker, or cronjob. Multi-region by default.     | Yes          |
| `bahriya_registry`        | Private container registry credentials, scoped to an org.                 | Yes          |
| `bahriya_secret`          | Encrypted secret value, mountable as env var in a container.              | Yes          |
| `bahriya_memcached`       | Managed Memcached instance, attachable to containers as a cache backend.  | Yes          |
| `bahriya_tls_bundle`      | TLS bundle (CA + cert + key), versioned with rotation history.            | Yes          |
| `bahriya_x509_cert`       | Standalone X.509 certificate (no key), versioned.                         | Yes          |
| `bahriya_gpg_keypair`     | GPG public + private keypair (OpenPGP armor), versioned.                  | Yes          |
| `bahriya_ssh_keypair`     | SSH public + private keypair, versioned.                                  | Yes          |
| `bahriya_encryption_key`  | Symmetric encryption key (AES, ChaCha20, etc.), versioned.                | Yes          |
| `bahriya_env_file`        | `KEY=VALUE` env file. Injected into containers via `envFrom`.             | Yes          |
| `bahriya_yaml_config`     | YAML config file, mounted into containers as a file.                      | Yes          |
| `bahriya_json_config`     | JSON config file, mounted into containers as a file.                      | Yes          |
| `bahriya_plain_config`    | Arbitrary text config file, mounted into containers as a file.            | Yes          |

Each vault/config item also has a corresponding `bahriya_project_<type>_attachment` resource that binds an item to a project. Once attached, the item can be referenced from a `bahriya_container` block to mount it inside the container.

All resource schemas are **code-generated** from the OpenAPI specs at `packages/bahriya-openapi/specs/v1/`.

### Minimum required fields

These are the fields you **must** set for a successful deploy. Omitting any of these will cause errors (API rejection or runtime health-check failures).

#### `bahriya_container` (HTTP)

| Field                  | Type   | Example        | Notes                                                          |
| ---------------------- | ------ | -------------- | -------------------------------------------------------------- |
| `handle`               | string | `"api"`        | Immutable. Cannot be reused after delete (soft-delete).        |
| `name`                 | string | `"API Server"` |                                                                |
| `image`                | string | `"nginx:alpine"` | Full image ref including tag.                                |
| `containerport`        | string | `"8080"`       | The port your application listens on.                          |
| `healthcheckpath`      | string | `"/healthz"`   | HTTP path for liveness/readiness probes. **Deploy will fail if the container does not respond 200 at this path on `containerport`.** |
| `mincpu`               | string | `"100"`        | Millicores.                                                    |
| `minmemory`            | string | `"128"`        | Megabytes.                                                     |
| `autoscalingminreplicas` | string | `"1"`        |                                                                |
| `activeregions`        | list   | `["falkenstein-1"]` | At least one region.                                     |
| `project`              | string | `bahriya_project.x.id` | Must be the project **UUID** (`.id`), not the handle.  |

#### `bahriya_container` (worker / cronjob)

Same as HTTP except `containerport` and `healthcheckpath` are not required (workers have no ingress).

#### `bahriya_memcached`

| Field           | Type   | Example              | Notes                                                   |
| --------------- | ------ | -------------------- | ------------------------------------------------------- |
| `handle`        | string | `"session-cache"`    | Immutable. Cannot be reused after delete.               |
| `name`          | string | `"Session Cache"`    |                                                         |
| `memorymb`      | number | `512`                | Cache size in megabytes (integer, not string).           |
| `activeregions` | list   | `["helsinki-1"]`     | At least one region.                                    |
| `project`       | string | `bahriya_project.x.id` | Must be the project **UUID** (`.id`), not the handle. |

#### `bahriya_project`

| Field    | Type   | Example              | Notes                       |
| -------- | ------ | -------------------- | --------------------------- |
| `handle` | string | `"web-prod"`         | Immutable. Soft-deleted.    |
| `name`   | string | `"Web Production"`   |                             |
| `regions`| list   | `["falkenstein-1"]`  | At least one region.        |

#### `bahriya_registry`

| Field      | Type   | Example          | Notes                                    |
| ---------- | ------ | ---------------- | ---------------------------------------- |
| `handle`   | string | `"ghcr"`         | Released on delete (reusable).           |
| `name`     | string | `"GHCR"`         |                                          |
| `server`   | string | `"ghcr.io"`      | Registry hostname.                       |
| `username` | string | `"myuser"`       |                                          |
| `password` | string | (sensitive)      |                                          |

#### `bahriya_secret`

| Field    | Type   | Example           | Notes                          |
| -------- | ------ | ----------------- | ------------------------------ |
| `handle` | string | `"db-password"`   | Released on delete (reusable). |
| `name`   | string | `"DB Password"`   |                                |
| `value`  | string | (sensitive)       |                                |

#### Vault items

All vault types require `handle` and `name` plus the content fields below. Handles are immutable. Items are soft-deleted (handle burned).

| Resource                 | Required content fields              | Notes                                                                 |
| ------------------------ | ------------------------------------ | --------------------------------------------------------------------- |
| `bahriya_tls_bundle`     | `ca`, `cert`, `key`                  | PEM-encoded. `key` is sensitive.                                      |
| `bahriya_x509_cert`      | `cert`                               | PEM-encoded standalone cert (no key).                                 |
| `bahriya_gpg_keypair`    | `public_key`, `private_key`          | OpenPGP armored. Both fields marked sensitive by the spec.            |
| `bahriya_ssh_keypair`    | `public_key`, `private_key`          | OpenSSH format. Both fields marked sensitive by the spec.             |
| `bahriya_encryption_key` | `algorithm`, `format`, `key`         | `format` ∈ `base64`, `hex`, `raw`. `key` is sensitive.                |

The API parses content on create / rotate — invalid material is rejected.

#### Config items

| Resource              | Required content fields | Notes                                                          |
| --------------------- | ----------------------- | -------------------------------------------------------------- |
| `bahriya_env_file`    | `content`               | `KEY=VALUE` lines. Injected via `envFrom`.                     |
| `bahriya_yaml_config` | `content`               | Valid YAML.                                                    |
| `bahriya_json_config` | `content`               | Valid JSON.                                                    |
| `bahriya_plain_config`| `content`               | Arbitrary text. Mounted as a file.                             |

#### Project attachments

Each vault/config type has a corresponding attachment resource (`bahriya_project_tls_bundle_attachment`, etc.). All take exactly two required fields:

| Field        | Type   | Description                                  |
| ------------ | ------ | -------------------------------------------- |
| `project_id` | string | UUID of the project (`bahriya_project.x.id`). |
| `handle`     | string | Handle of the item to attach.                |

Both attributes force replacement on change. Attachment is also blocked at the API if a container in the project still references the item.

## Data sources

| Data source              | Description                                                          |
| ------------------------ | -------------------------------------------------------------------- |
| `bahriya_organisation`   | Fetches the current organisation (id, name, handle, email, role).    |
| `bahriya_region`         | Look up a single region by ID (name, status, location).              |
| `bahriya_regions`        | List all regions, with optional `status_filter`.                     |

## Important behaviours

### Handles cannot be reused (soft-delete)

For **projects, containers, and memcached**, deleting in Terraform does not free up the handle. These resources are soft-deleted for billing and audit reasons.

- Use distinct handles per environment (`api-prod`, `api-staging`).
- Treat handles as **permanent identifiers**.

For **registries and secrets**, handles are released on delete and can be reused.

### Async termination

Destroying a `bahriya_container` or `bahriya_memcached` triggers async termination. The provider polls until the resource reaches `terminated` status (default timeout: 10 minutes).

### Renaming a handle is destroy + recreate

Handles are immutable. Changing the `handle` attribute forces Terraform to destroy and recreate the resource.

### Imports use UUIDs

```bash
terraform import bahriya_container.api 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```

### Sensitive fields

Registry `password` and secret `value` are marked sensitive. The API returns masked sentinels on read; the provider preserves the real value from your Terraform state.

Vault item content fields (`bahriya_tls_bundle.key`, `bahriya_gpg_keypair.private_key` / `.public_key`, `bahriya_ssh_keypair.private_key` / `.public_key`, `bahriya_encryption_key.key`) are also sensitive and follow the same masking pattern.

### Removing a nested attachment block must be explicit

Container nested-list attributes (`tls_bundles`, `x509_certs`, `gpg_keypairs`, `ssh_keypairs`, `encryption_keys`, `env_files`, `yaml_configs`, `json_configs`, `plain_configs`, `persistentvolumes`, `hostnames`, `initjobs`, `secretsenvvar`, `newenvvar`) are `Optional+Computed` with the `UseStateForUnknown` plan modifier — needed for stable plans against the API's PUT-replace semantics. As a consequence, **omitting a block in your `.tf` does NOT remove the attachment**; the prior state value is re-injected silently.

To remove a vault/config attachment from a container, set the block to an explicit empty list:

```hcl
resource "bahriya_container" "api" {
  # ...
  json_configs = []   # explicit removal — works
  # plain_configs    # omitting — does NOT remove; prior list is preserved
}
```

### Detach guard

Attempting to detach a vault/config item from a project while a container in that project still references it returns `409 CONFLICT` with the offending container's handle. Update the container to remove the reference first.

### Container Update waits for status

After a `terraform apply` that modifies a container, the provider waits for the container to reach `running` (or terminal `error`) before completing the apply. This avoids "inconsistent result after apply" errors when the post-PUT fetch would otherwise race the helm rollout.

### Common pitfalls

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| Container reaches `error` status after deploy | Missing or wrong `healthcheckpath`. The platform probes `containerport` + `healthcheckpath` to decide if the container is healthy. | Set `healthcheckpath` to a path your app responds 200 on (e.g. `/` or `/healthz`). |
| `project` attribute rejected | Passing the project handle instead of UUID. | Use `bahriya_project.x.id`, not `.handle`. |
| `terraform plan` shows unknown field | Using snake_case names (`container_port`, `min_cpu`). The provider uses flat lowercase names from the API. | Use `containerport`, `mincpu`, `minmemory`, `activeregions`, etc. |
| Handle already taken after destroy | Soft-deleted resources don't release handles. | Use a new handle (e.g. `api-v2`). |

## Examples

Runnable configs live under `examples/`:

| Directory          | What it demonstrates                                       |
| ------------------ | ---------------------------------------------------------- |
| `project-only/`    | Minimal project resource                                   |
| `registry-secret/` | Registry + secret with variable inputs                     |
| `data-sources/`    | All 3 data sources (organisation, region, regions)         |
| `container/`       | Full container deployment with registry, secrets, hostname |
| `memcached/`       | Cache instance inside a project                            |
| `complete/`        | Everything together                                        |

## Building from source

Requires [Go 1.23+](https://go.dev/dl/).

```bash
git clone <repo-url>
cd terraform-provider-bahriya
make build        # builds ./bin/terraform-provider-bahriya
make test         # runs all 63 unit tests
make generate     # regenerates resource code from OpenAPI specs
make verify       # generate + git diff (CI gate)
```

## Local development

Add a `dev_overrides` block to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "bahriya-cloud/bahriya" = "/path/to/terraform-provider-bahriya/bin"
  }
  direct {}
}
```

Then just `make build` and run `terraform plan/apply` — no `terraform init` needed between rebuilds.

Set credentials via environment variables:

```bash
export BAHRIYA_TOKEN="pat_..."
export BAHRIYA_ORGANISATION_ID="..."
# Optional: override API URL for dev
export BAHRIYA_API_URL="http://localhost:8080/console/v1"
```

## Architecture

This is a **code-generated** provider. The OpenAPI specs at `packages/bahriya-openapi/specs/v1/` are the source of truth. A Go generator reads those specs and emits Terraform resources with full CRUD, ImportState, diff short-circuit, and status polling.

### What's generated

- All 5 resource files (`internal/resources/*_resource.go`) — ~3,800 LOC total
- Resource registry (`internal/resources/registry.go`)
- Terraform schemas with nested attributes for complex types (Container has 6 nested object types)
- Plan modifiers (RequiresReplace for handles, UseStateForUnknown for computed fields)
- Sensitive field preservation (password, value)

### What's hand-written

- Provider scaffold (`internal/provider/`) — schema, Configure, env-var fallbacks
- HTTP client (`internal/client/`) — PAT auth, retries, typed errors
- Data sources (`internal/datasources/`) — organisation, region, regions
- Transforms (`internal/transforms/`) — envvar/secret dict-to-array, x-implies
- Lifecycle (`internal/lifecycle/`) — WaitForStatus with backoff
- Helpers (`internal/resources/helpers.go`) — small utilities for generated code
- Code generator (`cmd/generator/`) — parser, model, renderer, template

Generated files are committed to git. `make verify` fails CI if they drift.

## Releasing

The source of truth is on 1x.ax (`origin`). GitHub (`github`) is a mirror that feeds the Terraform Registry. Push to both:

```bash
# Push code
git push origin && git push github

# Push tags (triggers the GitHub Actions release workflow)
git push origin --tags && git push github --tags
```

To cut a release:

```bash
git tag v0.1.0
git push origin && git push origin --tags && git push github && git push github --tags
```

The GitHub Actions workflow runs GoReleaser, builds cross-platform binaries, signs the checksums with GPG, and publishes the release. The Terraform Registry picks it up automatically.

## Repository layout

```
terraform-provider-bahriya/
├── cmd/
│   ├── terraform-provider-bahriya/   # plugin entrypoint
│   └── generator/                    # codegen tool
│       ├── templates/                # text/template for resources
│       └── parser_test.go            # generator unit tests
├── internal/
│   ├── client/       # HTTP client + tests
│   ├── provider/     # provider block + tests
│   ├── resources/    # GENERATED — do not edit by hand
│   ├── datasources/  # data sources + tests
│   ├── transforms/   # envvar, secret, implies + tests
│   └── lifecycle/    # WaitForStatus + tests
├── examples/         # 6 runnable Terraform configs
├── Makefile
├── go.mod
└── .gitignore
```

## Testing

63 unit tests across all packages:

```bash
make test           # runs go test ./internal/... and cmd/generator
```

| Package              | Tests | Coverage                                           |
| -------------------- | ----- | -------------------------------------------------- |
| `cmd/generator`      | 16    | Parser, model helpers, type resolution, nested $ref |
| `internal/client`    | 10    | HTTP envelope, auth, retry, error types             |
| `internal/datasources` | 17 | Metadata, schema, configure, response parsing       |
| `internal/provider`  | 5     | Metadata, schema, resource counts, firstNonEmpty    |
| `internal/lifecycle` | 5     | Wait-for-status (target, terminal, timeout, cancel) |
| `internal/transforms`| 11    | Envvar, secret, implies round-trips                 |
