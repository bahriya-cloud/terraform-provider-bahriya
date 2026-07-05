---
page_title: "bahriya_project_ssh_keypair_attachment Resource - Bahriya"
subcategory: ""
description: |-
  Attaches an SSH keypair to a Bahriya project.
---

# bahriya_project_ssh_keypair_attachment (Resource)

Attaches an SSH keypair to a Bahriya project so containers in that project can use it. The attachable itself is org-scoped and defined by the `bahriya_ssh_keypair` resource; this resource is the project-level link between them.

Both `project_id` and `handle` force replacement — changing either detaches and re-attaches.

## Example Usage

```hcl
resource "bahriya_ssh_keypair" "deploy_key" {
  # ... bahriya_ssh_keypair configuration ...
}

resource "bahriya_project_ssh_keypair_attachment" "web_ssh_keypair" {
  project_id = bahriya_project.web.id
  handle     = bahriya_ssh_keypair.deploy_key.handle
}
```

## Schema

### Required

- `project_id` (String) ID (UUID) of the project to attach to. Changing this replaces the attachment.
- `handle` (String) Handle of the ssh keypair to attach. Changing this replaces the attachment.

### Read-Only

- `id` (String) Composite identifier of the attachment (`{project_id}:{handle}`).
- `join_id` (String) Identifier of the underlying project-attachment join row.

## Import

Attachments can be imported using the composite id `{project_id}:{handle}`:

```bash
terraform import bahriya_project_ssh_keypair_attachment.web_ssh_keypair \
  8a3d0000-1111-2222-3333-444444444444:deploy-key
```
