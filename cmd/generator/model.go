package main

type Resource struct {
	Name             string
	GoName           string
	HasStatus        bool
	ReadyStatus      string
	TerminatedStatus string
	URLBase          string
	URLItem          string
	HandleURL        string
	HandlePathName   string
	CollectionKey    string
	DeleteMethod     string
	DeleteURL        string
	ConfirmDelete    bool
	Attributes       []Attribute
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
		if a.Type == "list_string" || a.Type == "list_integer" || a.Type == "list_nested" {
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
	Type            string // "string", "int64", "bool", "list_string", "list_integer", "list_nested"
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
