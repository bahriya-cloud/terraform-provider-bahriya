---
page_title: "bahriya_path_rule Resource - Bahriya"
subcategory: ""
description: |-
  Path-scoped HTTP control plugin on a Bahriya container — basic auth, rate limit, IP allow/deny per URL prefix.
---

# bahriya_path_rule (Resource)

Apply basic authentication, rate limiting, or IP allow / deny lists to a specific URL prefix on an HTTP container. The longest matching path wins per request; a rule's controls fully override the container-wide settings for the same control type on that path.

## Example Usage

### Basic authentication on an admin area

```hcl
resource "bahriya_path_rule" "admin" {
  container_id = bahriya_container.api.id
  handle       = "admin"
  path         = "/api/admin"

  basicauthenabled = true
  basicauthcredentials = [
    {
      username = "alice"
      password = var.admin_password
    },
  ]
}
```

### Rate limiting on a webhook endpoint

```hcl
resource "bahriya_path_rule" "webhook" {
  container_id = bahriya_container.api.id
  handle       = "webhook"
  path         = "/webhook"

  ratelimitingenabled           = true
  ratelimitingrequestsperminute = 60
  ratelimitingrequestsperhour   = 1000
}
```

### Combined controls on an internal endpoint

```hcl
resource "bahriya_path_rule" "internal" {
  container_id = bahriya_container.api.id
  handle       = "internal-tools"
  path         = "/internal"
  priority     = 100

  ipwhitelistenabled = true
  ipwhitelist        = ["10.0.0.0/8", "192.168.0.0/16"]

  basicauthenabled = true
  basicauthcredentials = [
    {
      username = "ops"
      password = var.ops_password
    },
  ]
}
```

## Schema

### Required

- `container_id` (String) ID (UUID) of the HTTP container this rule attaches to. Changing this replaces the rule.
- `handle` (String) Handle for the rule. DNS-1123 compliant. Unique among active rules on the container. Changing it replaces the rule.
- `path` (String) URL path prefix. Must start with `/`. Longest matching prefix wins per request.

### Optional

- `priority` (Number) Tiebreaker when two rules share equal path-prefix length. Higher values win. Defaults to `0`.
- `basicauthenabled` (Boolean) Enable HTTP basic authentication on this path.
- `basicauthcredentials` (Attributes List) List of `{username, password}` credential pairs. The password is sensitive — the API masks it on read with the sentinel `value-hidden-for-your-own-good` so re-applying without rotating the password does not drift.
  - `username` (String, required)
  - `password` (String, required, sensitive)
- `ratelimitingenabled` (Boolean) Enable per-IP rate limiting on this path.
- `ratelimitingrequestspersecond` (Number) Max requests per second per IP.
- `ratelimitingrequestsperminute` (Number) Max requests per minute per IP.
- `ratelimitingrequestsperhour` (Number) Max requests per hour per IP.
- `ipwhitelistenabled` (Boolean) Enable IP allow-list.
- `ipwhitelist` (List of String) IP addresses or CIDR ranges allowed on this path.
- `ipblacklistenabled` (Boolean) Enable IP deny-list.
- `ipblacklist` (List of String) IP addresses or CIDR ranges blocked on this path.

### Read-Only

- `id` (String) UUID of the path rule.

## Import

Path rules can be imported using a composite id `<container_id>:<path_rule_id>`:

```bash
terraform import bahriya_path_rule.admin \
  c0ffee00-aaaa-bbbb-cccc-000000000001:dd0ffee0-1111-2222-3333-444444444444
```

After import, the basic auth password starts as the API's masked sentinel — running `terraform apply` with a real password rotates it.
