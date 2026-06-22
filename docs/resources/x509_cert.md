---
page_title: "bahriya_x509_cert Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya x509 certificate — a standalone PEM-encoded certificate (no key).
---

# bahriya_x509_cert (Resource)

Manages a Bahriya x509 certificate. Unlike `bahriya_tls_bundle`, this resource stores only a certificate (no private key) — useful for distributing CA bundles, client trust anchors, or signed leaf certs.

## Example Usage

```hcl
resource "bahriya_x509_cert" "client_ca" {
  handle = "client-ca"
  name   = "Client CA"
  cert   = file("${path.module}/client-ca.pem")
}
```

## Schema

### Required

- `handle` (String) - DNS-1123 compliant: lowercase alphanumeric and hyphens only. Immutable — changing forces recreation.
- `name` (String) - Display name.
- `cert` (String) - PEM-encoded x509 certificate.

### Optional

- `maxversions` (Number) - Maximum number of historical versions to retain.

### Read-Only

- `id` (String) - Certificate UUID.
- `algorithm` (String) - Signature algorithm.
- `billable` (Boolean) - Whether this certificate is billable.
- `currentversion` (Number) - The currently active version number.
- `fingerprint` (String) - Certificate fingerprint.
- `issuer` (String) - Certificate issuer DN.
- `managedbyresourceid` (String) - UUID of the managing resource, if any.
- `managedbyresourcetype` (String) - Type of the managing resource, if any.
- `organisation` (String) - Organisation UUID.
- `subject` (String) - Certificate subject DN.

## Import

```bash
terraform import bahriya_x509_cert.client_ca 065df92e-4e46-436a-a0a0-aaaaaaaaaaaa
```
