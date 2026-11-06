package main

import "strings"

// AttachmentResource is the template data for per-type project attachment
// resources (bahriya_project_<type>_attachment). These are deliberately thin —
// project_id + handle inputs, computed join_id, no Update.
type AttachmentResource struct {
	Name    string // e.g. "tls_bundle" — used in the terraform resource type suffix
	GoName  string // e.g. "TLSBundle" — used in Go identifiers
	APIType string // e.g. "tls_bundles" — URL path segment (snake_case)
}

// ListingKey is the key under which this attachable appears in the
// GET /organisations/{org}/projects/{pid}/attachments response. The response
// groups attachables by SMUSHED key (tls_bundles → tlsbundles), matching the
// Layer-3 wire convention — NOT the snake_case URL path segment. Using APIType
// here would make the attachment Read never match, silently dropping the
// resource from state and dead-locking terraform destroy.
func (a AttachmentResource) ListingKey() string {
	return strings.ReplaceAll(a.APIType, "_", "")
}
