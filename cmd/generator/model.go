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

type Resource struct {
	Name              string
	GoName            string
	HasStatus         bool
	ReadyStatus       string
	TerminatingStatus string
	TerminatedStatus  string
	URLBase           string
	URLItem           string
	HandleURL         string
	HandlePathName    string
	CollectionKey     string
	DeleteMethod      string
	DeleteURL         string
	ConfirmDelete     bool
	// DeleteConflictHint, when non-empty, is appended to the Delete error
	// when the API returns 409. Used to guide users on resources where a
	// 409 has a non-obvious resolution (e.g. project delete blocked by
	// child resources in non-terminated states).
	DeleteConflictHint string
	// OmitOrganisationInBody suppresses the redundant `organisation` create/update
	// body key. Org is always scoped by the URL path; most DTOs tolerate the extra
	// key but the Registry and Secret DTOs reject it (400 "Unexpected key
	// organisation"). Conversely Project *requires* it, so this is opt-in per type.
	OmitOrganisationInBody bool
	// Noun is how prose in generated diagnostics refers to one of these
	// (e.g. "instance" for datastores). Defaults to Name.
	Noun       string
	Attributes []Attribute
}

func (r Resource) FlatResponse() bool { return r.CollectionKey == "" }

func (r Resource) EffectiveDeleteMethod() string {
	if r.DeleteMethod != "" {
		switch r.DeleteMethod {
		case "POST":
			return "Post"
		case "DELETE":
			return "Delete"
		default:
			return r.DeleteMethod
		}
	}
	return "Delete"
}

func (r Resource) EffectiveDeleteURL() string {
	if r.DeleteURL != "" {
		return r.DeleteURL
	}
	return r.URLItem
}

func (r Resource) NeedsListPlanModifier() bool {
	for _, a := range r.Attributes {
		if (a.Type == "list_string" || a.Type == "list_integer" || a.Type == "list_nested") && (a.Computed || a.Optional) {
			return true
		}
	}
	return false
}

func (r Resource) NeedsMapPlanModifier() bool {
	for _, a := range r.Attributes {
		if a.Type == "map_string" && (a.Computed || a.Optional) {
			return true
		}
	}
	return false
}

func (r Resource) HasMapString() bool {
	for _, a := range r.Attributes {
		if a.Type == "map_string" {
			return true
		}
	}
	return false
}

func (r Resource) NeedsInt64PlanModifier() bool {
	for _, a := range r.Attributes {
		if a.Type == "int64" && (a.Computed || a.Optional) {
			return true
		}
	}
	return false
}

func (r Resource) HasSensitiveFields() bool {
	for _, a := range r.Attributes {
		if a.Sensitive {
			return true
		}
	}
	return false
}

func (r Resource) SensitiveAttributes() []Attribute {
	var out []Attribute
	for _, a := range r.Attributes {
		if a.Sensitive {
			out = append(out, a)
		}
	}
	return out
}

func (r Resource) HasNestedAttributes() bool {
	for _, a := range r.Attributes {
		if a.Type == "list_nested" {
			return true
		}
	}
	return false
}

func (r Resource) NestedAttributes() []Attribute {
	var out []Attribute
	for _, a := range r.Attributes {
		if a.Type == "list_nested" {
			out = append(out, a)
		}
	}
	return out
}

func (r Resource) NeedsAttrImport() bool {
	for _, a := range r.Attributes {
		if a.Type == "list_string" || a.Type == "list_integer" || a.Type == "list_nested" || a.Type == "map_string" {
			return true
		}
	}
	return false
}

func (r Resource) HasListInteger() bool {
	for _, a := range r.Attributes {
		if a.Type == "list_integer" {
			return true
		}
	}
	return false
}

type Attribute struct {
	TfName          string
	APIName         string
	GoName          string
	Type            string // "string", "int64", "bool", "list_string", "list_integer", "list_nested", "map_string"
	Required        bool
	Optional        bool
	Computed        bool
	RequiresReplace bool
	Sensitive       bool
	Description     string
	Nested          []NestedField // populated when Type == "list_nested"
}

type NestedField struct {
	TfName   string
	APIName  string
	GoName   string
	Type     string // "string", "int64", "bool", "list_string"
	Required bool
}

func (a Attribute) IsList() bool {
	return a.Type == "list_string" || a.Type == "list_integer" || a.Type == "list_nested"
}

func (a Attribute) IsNested() bool { return a.Type == "list_nested" }

func (a Attribute) TfsdkType() string {
	switch a.Type {
	case "string":
		return "types.String"
	case "int64":
		return "types.Int64"
	case "bool":
		return "types.Bool"
	case "list_string", "list_integer", "list_nested":
		return "types.List"
	case "map_string":
		return "types.Map"
	default:
		return "types.String"
	}
}

func (a Attribute) SchemaType() string {
	switch a.Type {
	case "string":
		return "schema.StringAttribute"
	case "int64":
		return "schema.Int64Attribute"
	case "bool":
		return "schema.BoolAttribute"
	case "list_string", "list_integer":
		return "schema.ListAttribute"
	case "list_nested":
		return "schema.ListNestedAttribute"
	case "map_string":
		return "schema.MapAttribute"
	default:
		return "schema.StringAttribute"
	}
}

func (f NestedField) TfsdkType() string {
	switch f.Type {
	case "string":
		return "types.String"
	case "int64":
		return "types.Int64"
	case "bool":
		return "types.Bool"
	case "list_string":
		return "types.List"
	default:
		return "types.String"
	}
}

func (f NestedField) SchemaType() string {
	switch f.Type {
	case "string":
		return "schema.StringAttribute"
	case "int64":
		return "schema.Int64Attribute"
	case "bool":
		return "schema.BoolAttribute"
	case "list_string":
		return "schema.ListAttribute"
	default:
		return "schema.StringAttribute"
	}
}

func (f NestedField) IsList() bool { return f.Type == "list_string" }
