---
page_title: "Provider: Bahriya"
subcategory: ""
description: |-
  The Bahriya provider manages every Bahriya Cloud resource.
---

# Bahriya Provider

The Bahriya provider lets you manage infrastructure on [Bahriya Cloud](https://bahriya.cloud) declaratively with Terraform. It supports every Bahriya resource — projects, containers, managed datastores, vault items, configs and network policies — so your entire estate can live in version control.

## About Bahriya

[Bahriya](https://bahriya.cloud) is a container cloud with a simple contract: bring a container image, pick your regions, and the platform runs it — load balancing, TLS, health checks, autoscaling, a secrets vault and managed datastores included rather than bolted on.

- [HTTP Containers](https://bahriya.cloud/product/containers) — TLS-terminated, autoscaling, multi-region services with custom hostnames.
- [Workers](https://bahriya.cloud/product/workers) and [Cron Jobs](https://bahriya.cloud/product/cronjobs) — background and scheduled containers.
- [Managed Valkey](https://bahriya.cloud/product/valkey) — the Redis-compatible, open-source in-memory datastore — and [Managed Memcached](https://bahriya.cloud/product/memcached).
- [Vault](https://bahriya.cloud/product/vault) and [Configs](https://bahriya.cloud/product/configs) — secrets, TLS bundles, keypairs, encryption keys and config files, versioned and mountable.
- [Network Policies](https://bahriya.cloud/product/network-policies) and [Volume Storage](https://bahriya.cloud/product/volume-storage) — traffic control and persistence.

Every resource in this provider has a matching [Terraform knowledgebase guide](https://bahriya.cloud/knowledgebase/terraform) with a complete, working deploy example. New to the platform? Start with [getting started](https://bahriya.cloud/knowledgebase/getting-started), browse the [full knowledgebase](https://bahriya.cloud/knowledgebase), or follow launches on [the blog](https://bahriya.cloud/blog).

## Authentication

The provider authenticates with a **personal access token** (PAT):

1. Sign in to the [Bahriya console](https://app.bahriya.cloud).
2. Open your profile, then **Tokens**.
3. Click **Create token** and copy the `pat_...` value.

Export the token and your organisation ID:

```bash
export BAHRIYA_TOKEN="pat_..."
export BAHRIYA_ORGANISATION_ID="65cc42f1-..."
```

Or set them in the provider block:

```hcl
provider "bahriya" {
  token           = var.bahriya_token
  organisation_id = var.bahriya_org_id
}
```

## Example Usage

```hcl
terraform {
  required_providers {
    bahriya = {
      source  = "bahriya-cloud/bahriya"
      version = "~> 0.3"
    }
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

resource "bahriya_container" "api" {
  handle          = "api"
  name            = "API Server"
  image           = "ghcr.io/myorg/api:1.2.3"
  containerport   = "3000"
  healthcheckpath = "/healthz"
  mincpu          = "250"
  minmemory       = "256"

  autoscalingminreplicas = "2"
  activeregions          = [data.bahriya_regions.active.regions[0].id]
  project                = bahriya_project.web.id
}
```

## Multi-Organisation Setup

Use provider aliases to manage resources across organisations:

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

## Schema

### Optional

- `token` (String, Sensitive) - Personal access token. Falls back to `BAHRIYA_TOKEN` environment variable.
- `organisation_id` (String) - Organisation UUID. Falls back to `BAHRIYA_ORGANISATION_ID` environment variable.
- `base_url` (String) - API base URL. Defaults to `https://api.bahriya.cloud/console/v1`. Falls back to `BAHRIYA_API_URL` environment variable.
