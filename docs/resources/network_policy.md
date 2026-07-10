---
page_title: "bahriya_network_policy Resource - Bahriya"
subcategory: ""
description: |-
  Manages a Bahriya network policy — reusable ingress/egress traffic control attached to projects and deployables.
---

# bahriya_network_policy (Resource)

Manages a Bahriya network policy. A network policy is an org-scoped, reusable rule set controlling which traffic is allowed to and from the workloads it is attached to. Attach it project-wide with `bahriya_project_network_policy_attachment`, or narrow it to a single container/memcached via that resource's `networkpolicies` attribute.

By default a project's workloads can talk to each other and reach the internet. Attaching a policy tightens that: ingress is limited to the listed peer projects, and once any `egresscidrs` (or L7 `egressfqdns`) are set, egress switches to deny-by-default plus the listed allowlist (DNS always stays allowed).

## Example Usage

```hcl
resource "bahriya_network_policy" "web_tier" {
  handle = "web-tier"
  name   = "Web tier"

  # Ingress: only these same-org projects may reach the targets.
  ingresspeers = ["frontend", "gateway"]

  # Egress (L3/L4): outbound restricted to these CIDRs on the listed ports.
  egresscidrs = ["10.0.0.0/8", "203.0.113.0/24"]

  ports = [
    { port = 443, protocol = "TCP" },
    { port = 53, protocol = "UDP" },
  ]
}

# L7 egress: allow specific domains. Setting egressfqdns auto-enables l7enabled,
# which adds a per-replica charge for every container the policy is applied to.
resource "bahriya_network_policy" "notifications" {
  handle       = "notifications-to-partner"
  name         = "Notifications to partner"
  egressfqdns  = ["api.partner.example.com"]
}
```

## Schema

### Required

- `handle` (String) - DNS-1123 compliant: lowercase alphanumeric and hyphens only. Immutable — changing forces recreation.
- `name` (String) - Display name.

### Optional

- `ingresspeers` (List of String) - Same-organisation project handles allowed to reach this policy's targets. Omit to leave ingress unchanged from the namespace default.
- `egresscidrs` (List of String) - CIDR ranges the targets are allowed to send outbound traffic to. Adding any range switches egress to deny-by-default; DNS resolution always stays allowed.
- `egressfqdns` (List of String) - Domain names (FQDNs) allowed for outbound traffic. These are enforced at L7, so listing any domain automatically enables `l7enabled` (and its per-replica charge).
- `l7enabled` (Boolean) - Enable L7 (application-layer) controls for this policy. Opts the policy into the application-layer engine and its per-replica charge for every container it is applied to. Automatically enabled when `egressfqdns` are set. Has no effect on datastores (memcached), which are L3/L4 only.
- `ports` (Attributes List) - Optional port/protocol scoping applied to the rules above. Omit to allow all ports. (see [below for nested schema](#nestedatt--ports))

### Read-Only

- `id` (String) - Network policy UUID.
- `organisation` (String) - Organisation UUID.
- `billable` (Boolean) - Whether this policy is billable.
- `managedbyresourceid` (String) - Set when the policy is managed by another resource; empty for user-created policies.
- `managedbyresourcetype` (String) - Type of the managing resource, when managed.

<a id="nestedatt--ports"></a>
### Nested Schema for `ports` (Attributes List)

Assign with attribute syntax, e.g. `ports = [{ port = 443, protocol = "TCP" }]`.

- `port` (Number, Required) - Port number the rule applies to.
- `protocol` (String, Optional) - `TCP` (default) or `UDP`.

## Import

Network policies can be imported using their UUID:

```bash
terraform import bahriya_network_policy.web_tier 8a3d0000-1111-2222-3333-444444444444
```
