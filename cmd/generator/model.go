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

// attachmentNouns is how customer-facing descriptions name each attachable,
// with its indefinite article.
var attachmentNouns = map[string][2]string{
	"registry":       {"a", "registry"},
	"secret":         {"a", "secret"},
	"tls_bundle":     {"a", "TLS bundle"},
	"x509_cert":      {"an", "x509 certificate"},
	"gpg_keypair":    {"a", "GPG keypair"},
	"ssh_keypair":    {"an", "SSH keypair"},
	"encryption_key": {"an", "encryption key"},
	"env_file":       {"an", "env file"},
	"yaml_config":    {"a", "YAML config"},
	"json_config":    {"a", "JSON config"},
	"plain_config":   {"a", "plain-text config"},
	"network_policy": {"a", "network policy"},
}

// Noun is the attachable's name in descriptions ("TLS bundle").
func (a AttachmentResource) Noun() string {
	return attachmentNouns[a.Name][1]
}

// Article is the indefinite article for Noun ("a" or "an").
func (a AttachmentResource) Article() string {
	return attachmentNouns[a.Name][0]
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
