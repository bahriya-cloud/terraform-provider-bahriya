---
page_title: "bahriya_ssh_keypair Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya SSH keypair — a PEM-encoded private key with its matching public key line.
---

# bahriya_ssh_keypair (Resource)

Manages a Bahriya SSH keypair. The private key is PEM-encoded and the public key is a standard SSH public-key line (`algorithm base64 comment`). The private key is encrypted at rest and is never returned on read.

## Example Usage

```hcl
resource "bahriya_ssh_keypair" "deploy" {
  handle      = "deploy"
  name        = "Deploy Key"
  public_key  = file("${path.module}/deploy.pub")
  private_key = file("${path.module}/deploy")
}
```

## Schema

### Required

- `handle` (String) - DNS-1123 compliant: lowercase alphanumeric and hyphens only. Immutable — changing forces recreation.
- `name` (String) - Display name.
- `public_key` (String, Sensitive) - SSH public key line (algorithm base64 comment).
- `private_key` (String, Sensitive) - PEM-encoded private key. Never returned on read.

### Optional

- `maxversions` (Number) - Maximum number of historical versions to retain.

### Read-Only

- `id` (String) - Keypair UUID.
- `algorithm` (String) - Key algorithm (e.g. ssh-ed25519, ssh-rsa).
- `billable` (Boolean) - Whether this keypair is billable.
- `comment` (String) - The comment portion of the public key line, if present.
- `currentversion` (Number) - The currently active version number.
- `key_bits` (Number) - Key length in bits.
- `key_id` (String) - The SSH key fingerprint.
- `managedbyresourceid` (String) - UUID of the managing resource, if any.
- `managedbyresourcetype` (String) - Type of the managing resource, if any.
- `organisation` (String) - Organisation UUID.

## Import

```bash
terraform import bahriya_ssh_keypair.deploy 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```

~> **Note:** Keys are never returned by the API. After importing, you must set `public_key` and `private_key` in configuration; the provider preserves the values from Terraform state.
