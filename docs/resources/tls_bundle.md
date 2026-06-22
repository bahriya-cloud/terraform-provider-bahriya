---
page_title: "bahriya_tls_bundle Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya TLS bundle — a CA certificate, server certificate and private key stored together for use by containers.
---

# bahriya_tls_bundle (Resource)

Manages a Bahriya TLS bundle. Each bundle holds a CA, a server certificate and the matching private key. The private key is encrypted at rest and is never returned on read.

## Example Usage

```hcl
resource "bahriya_tls_bundle" "edge" {
  handle = "edge-tls"
  name   = "Edge TLS"
  ca     = file("${path.module}/ca.pem")
  cert   = file("${path.module}/server.pem")
  key    = file("${path.module}/server.key")
}
```

## Schema

### Required

- `handle` (String) - DNS-1123 compliant: lowercase alphanumeric and hyphens only. Immutable — changing forces recreation.
- `name` (String) - Display name.
- `ca` (String) - PEM-encoded CA certificate.
- `cert` (String) - PEM-encoded server certificate.
- `key` (String, Sensitive) - PEM-encoded private key. Never returned on read.

### Optional

- `maxversions` (Number) - Maximum number of historical versions to retain.

### Read-Only

- `id` (String) - Bundle UUID.
- `algorithm` (String) - Detected key algorithm.
- `billable` (Boolean) - Whether this bundle is billable.
- `currentversion` (Number) - The currently active version number.
- `fingerprint` (String) - Certificate fingerprint.
- `issuer` (String) - Certificate issuer DN.
- `keybits` (Number) - Key length in bits.
- `managedbyresourceid` (String) - UUID of the managing resource, if any.
- `managedbyresourcetype` (String) - Type of the managing resource, if any.
- `organisation` (String) - Organisation UUID.
- `subject` (String) - Certificate subject DN.

## Import

```bash
terraform import bahriya_tls_bundle.edge 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```

~> **Note:** The `key` attribute is never returned by the API. After importing, you must set it in configuration; the provider preserves the value from Terraform state.
