---
page_title: "bahriya_tls_bundle Data Source - Bahriya"
subcategory: ""
description: |-
  Fetches a TLS bundle from the organisation's vault by handle, including its CA certificate and certificate.
---

# bahriya_tls_bundle (Data Source)

Fetches a TLS bundle from the organisation's vault by handle, including its CA certificate and certificate. The private key is never returned.

Its main use is the platform-managed certificate of a `bahriya_mysql` or `bahriya_valkey` instance with TLS on and no `tlsbundle` of your own. That bundle's handle is the instance handle followed by `-tls`. Clients that verify the server (MySQL `ssl-mode=VERIFY_IDENTITY`, Valkey `--cacert`) need its `ca`. Renewals keep the same CA, so a saved copy keeps working. A CA replacement, requested from the console or with `reis mysql:replace-ca`, changes `ca` on the next refresh.

## Example Usage

```hcl
resource "bahriya_mysql" "orders" {
  name          = "Orders DB"
  handle        = "orders-db"
  tier          = "single"
  cpumillicores = 500
  memorymb      = 1024
  storagegb     = 10
  project       = bahriya_project.shop.id
  activeregions = ["helsinki-1"]
  databases     = [{ name = "orders", username = "orders-app" }]
  tlsenabled    = true
}

data "bahriya_tls_bundle" "orders" {
  handle = bahriya_mysql.orders.tlsbundle
}

# The CA certificate clients use to verify the server, for example:
# terraform output -raw orders_ca > orders-db-ca.pem
output "orders_ca" {
  value = data.bahriya_tls_bundle.orders.ca
}
```

## Schema

### Required

- `handle` (String) - Handle of the TLS bundle.

### Read-Only

- `id` (String) - TLS bundle UUID.
- `name` (String) - TLS bundle display name.
- `ca` (String) - CA certificate (PEM). Clients verifying the server trust this.
- `cert` (String) - Certificate (PEM).
- `subject` (String) - Certificate subject.
- `issuer` (String) - Certificate issuer.
- `fingerprint` (String) - Certificate fingerprint.
- `expires` (String) - Certificate expiry time.
- `managed` (Boolean) - Whether the platform manages the bundle on behalf of a resource (for example a MySQL or Valkey instance).
