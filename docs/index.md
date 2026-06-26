---
page_title: "Provider: Bahriya"
subcategory: ""
description: |-
  The Bahriya provider manages every Bahriya Cloud resource.
---

# Bahriya Provider

The Bahriya provider lets you manage infrastructure on [Bahriya Cloud](https://bahriya.cloud) declaratively with Terraform. It supports every Bahriya resource.

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
      version = "~> 0.1"
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
