---
page_title: "bahriya_encryption_key Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya symmetric encryption key (AES, ChaCha20, etc.).
---

# bahriya_encryption_key (Resource)

Manages a Bahriya symmetric encryption key. The key material is encrypted at rest and is never returned on read.

## Example Usage

```hcl
resource "bahriya_encryption_key" "tokens" {
  handle    = "session-tokens"
  name      = "Session token AES key"
  format    = "base64"
  key       = var.session_token_key
}
```

## Changing the key

A new key is a rotation, which this provider version does not perform. Changing `key` fails at plan time with "Rotation is not supported by this provider version". Rotate the key with `reis encryption_key:rotate` or in the console, then set the new value in your configuration. Renaming (`name`) is applied in place. Terraform provider v1 rotates the material for you.

`format` is fixed when a key is created. Changing it fails at plan time; to use another format, create a new key and move its attachments.

## Schema

### Required

- `handle` (String) - DNS-1123 compliant: lowercase alphanumeric and hyphens only. Immutable — changing forces recreation.
- `name` (String) - Display name.
- `format` (String) - Key encoding format: `base64`, `hex`, or `raw`.
- `key` (String, Sensitive) - The raw encryption key (base64 or hex encoded). Never returned on read.

### Optional

- `maxversions` (Number) - Maximum number of historical versions to retain.

### Read-Only

- `id` (String) - Key UUID.
- `billable` (Boolean) - Whether this key is billable.
- `currentversion` (Number) - The currently active version number.
- `keybits` (Number) - Key length in bits.
- `managedbyresourceid` (String) - UUID of the managing resource, if any.
- `managedbyresourcetype` (String) - Type of the managing resource, if any.
- `organisation` (String) - Organisation UUID.
- `versioncount` (Number) - How many versions are kept, the current one included.

## Import

```bash
terraform import bahriya_encryption_key.tokens 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```

~> **Note:** The `key` attribute is never returned by the API. After importing, you must set it in configuration; the provider preserves the value from Terraform state.
