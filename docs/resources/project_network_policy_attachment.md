---
page_title: "bahriya_project_network_policy_attachment Resource - Bahriya"
subcategory: ""
description: |-
  Attaches a network policy to a Bahriya project.
---

# bahriya_project_network_policy_attachment (Resource)

Attaches a network policy to a Bahriya project so it applies namespace-wide — to every container and datastore in that project. The policy itself is org-scoped and defined by the `bahriya_network_policy` resource; this resource is the project-level link between them.

To narrow a policy to a single deployable instead of the whole project, don't attach it here — reference it from the `networkpolicies` attribute on `bahriya_container` or `bahriya_memcached`. Project scope and per-container scope are mutually exclusive for the same policy: attaching a policy project-wide supersedes any per-container attachment of it in that project.

Both `project_id` and `handle` force replacement — changing either detaches and re-attaches.

## Example Usage

```hcl
resource "bahriya_network_policy" "web_tier" {
  # ... bahriya_network_policy configuration ...
}

resource "bahriya_project_network_policy_attachment" "web_web_tier" {
  project_id = bahriya_project.web.id
  handle     = bahriya_network_policy.web_tier.handle
}
```

## Schema

### Required

- `project_id` (String) - ID (UUID) of the project to attach to. Changing this replaces the attachment.
- `handle` (String) - Handle of the network policy to attach. Changing this replaces the attachment.

### Read-Only

- `id` (String) - Composite identifier of the attachment (`{project_id}:{handle}`).
- `join_id` (String) - Identifier of the underlying project-attachment join row.

## Import

Attachments can be imported using the composite id `{project_id}:{handle}`:

```bash
terraform import bahriya_project_network_policy_attachment.web_web_tier \
  8a3d0000-1111-2222-3333-444444444444:web-tier
```
