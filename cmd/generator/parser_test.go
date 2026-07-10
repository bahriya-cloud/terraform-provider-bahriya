package main

import (
	"testing"
)

func TestSnakeCase(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"handle", "handle"},
		{"minCPU", "min_c_p_u"},
		{"containerPort", "container_port"},
		{"activeRegions", "active_regions"},
		{"id", "id"},
		{"", ""},
		{"autoscalingEnabled", "autoscaling_enabled"},
	}
	for _, tt := range tests {
		got := snakeCase(tt.in)
		if got != tt.want {
			t.Errorf("snakeCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPascalCase(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"handle", "Handle"},
		{"container_port", "ContainerPort"},
		{"id", "ID"},
		{"cpu", "CPU"},
		{"base_url", "BaseURL"},
		{"ttl", "TTL"},
		{"min_mb", "MinMB"},
	}
	for _, tt := range tests {
		got := pascalCase(tt.in)
		if got != tt.want {
			t.Errorf("pascalCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPrimitiveType(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"string", "string", "string"},
		{"integer", "integer", "integer"},
		{"boolean", "boolean", "boolean"},
		{"array", "array", "array"},
		{"nil", nil, ""},
		{"nullable string", []any{"string", "null"}, "string"},
		{"nullable integer", []any{"integer", "null"}, "integer"},
		{"only null", []any{"null"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := primitiveType(tt.in)
			if got != tt.want {
				t.Errorf("primitiveType(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestResolveRef(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"#/components/schemas/Hostname", "Hostname"},
		{"#/components/schemas/NewEnvVariable", "NewEnvVariable"},
		{"invalid/ref", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := resolveRef(tt.in)
		if got != tt.want {
			t.Errorf("resolveRef(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAttrBand(t *testing.T) {
	tests := []struct {
		name string
		attr Attribute
		want int
	}{
		{"id", Attribute{TfName: "id"}, 0},
		{"handle", Attribute{TfName: "handle"}, 1},
		{"required", Attribute{TfName: "name", Required: true}, 2},
		{"optional", Attribute{TfName: "desc", Optional: true}, 3},
		{"computed", Attribute{TfName: "status", Computed: true}, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := attrBand(tt.attr)
			if got != tt.want {
				t.Errorf("attrBand(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestPlural(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"project", "projects"},
		{"registry", "registries"},
		{"secret", "secrets"},
		{"memcached", "memcached"},
		{"container", "containers"},
		{"network_policy", "networkpolicies"},
	}
	for _, tt := range tests {
		got := plural(tt.in)
		if got != tt.want {
			t.Errorf("plural(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAPISlug(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"registry", "registries"},
		{"secret", "secrets"},
		{"tls_bundle", "tls_bundles"},
		{"network_policy", "network_policies"},
	}
	for _, tt := range tests {
		got := apiSlug(tt.in)
		if got != tt.want {
			t.Errorf("apiSlug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolveAttrType_Primitives(t *testing.T) {
	schemas := map[string]rawSchema{}

	tests := []struct {
		name string
		prop rawProperty
		want string
	}{
		{"string", rawProperty{Type: "string"}, "string"},
		{"integer", rawProperty{Type: "integer"}, "int64"},
		{"boolean", rawProperty{Type: "boolean"}, "bool"},
		{"array of strings", rawProperty{Type: "array", Items: &rawProperty{Type: "string"}}, "list_string"},
		{"array of integers", rawProperty{Type: "array", Items: &rawProperty{Type: "integer"}}, "list_integer"},
		{"array no items", rawProperty{Type: "array"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := resolveAttrType(tt.prop, schemas)
			if got != tt.want {
				t.Errorf("resolveAttrType(%s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestResolveAttrType_NestedRef(t *testing.T) {
	schemas := map[string]rawSchema{
		"Hostname": {
			Required: []string{"hostname"},
			Properties: map[string]rawProperty{
				"hostname": {Type: "string"},
				"path":     {Type: "string"},
			},
		},
	}

	prop := rawProperty{
		Type: "array",
		Items: &rawProperty{
			Ref: "#/components/schemas/Hostname",
		},
	}
	typ, nested := resolveAttrType(prop, schemas)
	if typ != "list_nested" {
		t.Fatalf("expected list_nested, got %q", typ)
	}
	if len(nested) != 2 {
		t.Fatalf("expected 2 nested fields, got %d", len(nested))
	}
	hostFound := false
	for _, f := range nested {
		if f.APIName == "hostname" {
			hostFound = true
			if !f.Required {
				t.Error("hostname should be required")
			}
			if f.Type != "string" {
				t.Errorf("hostname type = %q, want string", f.Type)
			}
		}
	}
	if !hostFound {
		t.Error("hostname field not found in nested fields")
	}
}

func TestResource_FlatResponse(t *testing.T) {
	flat := Resource{CollectionKey: ""}
	if !flat.FlatResponse() {
		t.Error("expected FlatResponse=true for empty CollectionKey")
	}
	collection := Resource{CollectionKey: "projects"}
	if collection.FlatResponse() {
		t.Error("expected FlatResponse=false for CollectionKey=projects")
	}
}

func TestResource_EffectiveDeleteMethod(t *testing.T) {
	tests := []struct {
		method, want string
	}{
		{"", "Delete"},
		{"POST", "Post"},
		{"DELETE", "Delete"},
	}
	for _, tt := range tests {
		r := Resource{DeleteMethod: tt.method}
		got := r.EffectiveDeleteMethod()
		if got != tt.want {
			t.Errorf("EffectiveDeleteMethod(%q) = %q, want %q", tt.method, got, tt.want)
		}
	}
}

func TestResource_HasSensitiveFields(t *testing.T) {
	r := Resource{Attributes: []Attribute{
		{TfName: "name"},
		{TfName: "password", Sensitive: true},
	}}
	if !r.HasSensitiveFields() {
		t.Error("expected HasSensitiveFields=true")
	}
	r2 := Resource{Attributes: []Attribute{{TfName: "name"}}}
	if r2.HasSensitiveFields() {
		t.Error("expected HasSensitiveFields=false")
	}
}

func TestResource_NeedsListPlanModifier(t *testing.T) {
	r := Resource{Attributes: []Attribute{
		{TfName: "regions", Type: "list_string", Optional: true},
	}}
	if !r.NeedsListPlanModifier() {
		t.Error("expected NeedsListPlanModifier=true")
	}
	r2 := Resource{Attributes: []Attribute{
		{TfName: "regions", Type: "list_string", Required: true},
	}}
	if r2.NeedsListPlanModifier() {
		t.Error("expected NeedsListPlanModifier=false for Required-only list")
	}
}

func TestAttribute_IsList(t *testing.T) {
	for _, typ := range []string{"list_string", "list_integer", "list_nested"} {
		a := Attribute{Type: typ}
		if !a.IsList() {
			t.Errorf("IsList(%s) should be true", typ)
		}
	}
	a := Attribute{Type: "string"}
	if a.IsList() {
		t.Error("IsList(string) should be false")
	}
}

func TestAttribute_SchemaType(t *testing.T) {
	tests := []struct {
		typ, want string
	}{
		{"string", "schema.StringAttribute"},
		{"int64", "schema.Int64Attribute"},
		{"bool", "schema.BoolAttribute"},
		{"list_string", "schema.ListAttribute"},
		{"list_integer", "schema.ListAttribute"},
		{"list_nested", "schema.ListNestedAttribute"},
	}
	for _, tt := range tests {
		a := Attribute{Type: tt.typ}
		got := a.SchemaType()
		if got != tt.want {
			t.Errorf("SchemaType(%q) = %q, want %q", tt.typ, got, tt.want)
		}
	}
}

func TestIsCreateOnly(t *testing.T) {
	p := rawProperty{Extensions: map[string]any{"x-createOnly": true}}
	if !p.isCreateOnly() {
		t.Error("expected isCreateOnly=true")
	}
	p2 := rawProperty{Extensions: map[string]any{}}
	if p2.isCreateOnly() {
		t.Error("expected isCreateOnly=false without extension")
	}
	p3 := rawProperty{}
	if p3.isCreateOnly() {
		t.Error("expected isCreateOnly=false with nil extensions")
	}
}
