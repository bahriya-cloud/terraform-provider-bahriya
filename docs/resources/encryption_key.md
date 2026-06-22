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
  algorithm = "AES-256"
  format    = "base64"
  key       = var.session_token_key
}
```

## Schema

### Required

- `handle` (String) - DNS-1123 compliant: lowercase alphanumeric and hyphens only. Immutable — changing forces recreation.
- `name` (String) - Display name.
- `algorithm` (String) - Algorithm name (AES-128, AES-256, ChaCha20, etc.).
- `format` (String) - Key encoding format: `base64`, `hex`, or `raw`.
- `key` (String, Sensitive) - The raw encryption key (base64 or hex encoded). Never returned on read.

### Optional

- `maxversions` (Number) - Maximum number of historical versions to retain.

### Read-Only

- `id` (String) - Key UUID.
- `billable` (Boolean) - Whether this key is billable.
- `currentversion` (Number) - The currently active version number.
- `key_bits` (Number) - Key length in bits.
- `managedbyresourceid` (String) - UUID of the managing resource, if any.
- `managedbyresourcetype` (String) - Type of the managing resource, if any.
- `organisation` (String) - Organisation UUID.

## Import

```bash
terraform import bahriya_encryption_key.tokens 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```

~> **Note:** The `key` attribute is never returned by the API. After importing, you must set it in configuration; the provider preserves the value from Terraform state.
