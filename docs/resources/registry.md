---
page_title: "bahriya_registry Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya registry credential for pulling private container images.
---

# bahriya_registry (Resource)

Manages a Bahriya registry credential. Registries hold the credentials used to pull private container images. Reference a registry from a container via the `registry` attribute.

Unlike projects and containers, registry handles **are released** on delete and can be reused.

## Example Usage

### GitHub Container Registry

```terraform
resource "bahriya_registry" "ghcr" {
  handle   = "ghcr"
  name     = "GitHub Container Registry"
  server   = "ghcr.io"
  username = var.ghcr_username
  password = var.ghcr_token
}
```

### Docker Hub

```terraform
resource "bahriya_registry" "docker_hub" {
  handle   = "docker-hub"
  name     = "Docker Hub"
  server   = "registry-1.docker.io"
  username = var.docker_username
  password = var.docker_password
}
```

### AWS ECR

```terraform
resource "bahriya_registry" "ecr" {
  handle   = "ecr"
  name     = "AWS ECR"
  server   = "123456789.dkr.ecr.us-east-1.amazonaws.com"
  username = "AWS"
  password = var.ecr_token
}
```

### Referencing from a Container

```terraform
resource "bahriya_container" "api" {
  handle   = "api"
  name     = "API Server"
  image    = "ghcr.io/myorg/api:1.0.0"
  registry = bahriya_registry.ghcr.handle
  # ...
}
```

## Schema

### Required

- `handle` (String) - Unique lowercase handle. Immutable — changing this forces recreation.
- `name` (String) - Display name.
- `server` (String) - Registry server hostname (e.g. `ghcr.io`, `registry-1.docker.io`).
- `username` (String) - Registry username.
- `password` (String, Sensitive) - Registry password or access token.

### Optional

- `maxversions` (Number) - Maximum number of historical versions to retain.

### Read-Only

- `id` (String) - Registry UUID.
- `billable` (Boolean) - Whether this registry is billable.
- `currentversion` (Number) - The currently active version number.
- `currentversionid` (String) - UUID of the currently active version.
- `managedbyresourceid` (String) - UUID of the managing resource, if any.
- `managedbyresourcetype` (String) - Type of the managing resource, if any.
- `organisation` (String) - Organisation UUID.
- `versioncount` (Number) - How many versions are kept, the current one included.

## Import

Registries can be imported using their UUID:

```bash
terraform import bahriya_registry.ghcr 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
