---
page_title: "bahriya_gpg_keypair Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya GPG (PGP) keypair — ASCII-armored public and private keys stored together.
---

# bahriya_gpg_keypair (Resource)

Manages a Bahriya GPG keypair. Both keys are stored ASCII-armored. The private key is encrypted at rest and is never returned on read.

## Example Usage

```hcl
resource "bahriya_gpg_keypair" "signing" {
  handle      = "release-signing"
  name        = "Release Signing Key"
  public_key  = file("${path.module}/signing.pub.asc")
  private_key = file("${path.module}/signing.sec.asc")
}
```

## Schema

### Required

- `handle` (String) - DNS-1123 compliant: lowercase alphanumeric and hyphens only. Immutable — changing forces recreation.
- `name` (String) - Display name.
- `public_key` (String, Sensitive) - ASCII-armored PGP public key.
- `private_key` (String, Sensitive) - ASCII-armored PGP private key. Never returned on read.

### Optional

- `maxversions` (Number) - Maximum number of historical versions to retain.

### Read-Only

- `id` (String) - Keypair UUID.
- `algorithm` (String) - Key algorithm (RSA, EdDSA, etc.).
- `billable` (Boolean) - Whether this keypair is billable.
- `currentversion` (Number) - The currently active version number.
- `key_bits` (Number) - Key length in bits.
- `key_id` (String) - The PGP key id.
- `managedbyresourceid` (String) - UUID of the managing resource, if any.
- `managedbyresourcetype` (String) - Type of the managing resource, if any.
- `organisation` (String) - Organisation UUID.
- `uid` (String) - The primary UID on the key.

## Import

```bash
terraform import bahriya_gpg_keypair.signing 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```

~> **Note:** Keys are never returned by the API. After importing, you must set `public_key` and `private_key` in configuration; the provider preserves the values from Terraform state.
