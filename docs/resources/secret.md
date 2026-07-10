---
page_title: "bahriya_secret Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya secret — an encrypted value injected into containers as an environment variable.
---

# bahriya_secret (Resource)

Manages a Bahriya secret. Secrets are encrypted at rest and injected into containers as environment variables using the `secretsenvvar` block.

Unlike projects and containers, secret handles **are released** on delete and can be reused.

## Example Usage

### Create a Secret

```hcl
resource "bahriya_secret" "db_password" {
  handle = "db-password"
  name   = "Database Password"
  value  = var.db_password
}
```

### Inject into a Container

```hcl
resource "bahriya_container" "api" {
  handle = "api"
  name   = "API Server"
  image  = "myorg/api:1.0.0"
  # ...

  secretsenvvar = [
    { name = "DATABASE_PASSWORD", secret = bahriya_secret.db_password.handle },
    { name = "API_KEY", secret = bahriya_secret.api_key.handle },
  ]
}
```

## Schema

### Required

- `handle` (String) - Unique lowercase handle. Immutable — changing this forces recreation.
- `name` (String) - Display name.
- `value` (String, Sensitive) - The secret value. Encrypted at rest. The API returns a masked sentinel on read; the provider preserves the real value from your Terraform state.

### Read-Only

- `id` (String) - Secret UUID.
- `organisation` (String) - Organisation UUID.

## Import

Secrets can be imported using their UUID:

```bash
terraform import bahriya_secret.db_password 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```

~> **Note:** After importing, you must set the `value` attribute in your configuration. The API does not return secret values — only a masked sentinel.
