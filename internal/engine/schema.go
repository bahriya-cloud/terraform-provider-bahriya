// Package engine is the Terraform provider v1 resource engine (TF v1 #2247).
// One generic resource serves every kind: its schema is built from the kind's
// entry in resource-schema.json (internal/bmap), its plan is resolved by
// ModifyPlan with the canonical mapping rules (RULES.md), and its CRUD calls
// the console API as the v0.x generated resources did.
package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/bmap"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ReservedRootNames are the names Terraform reserves at the top level of every
// resource block. A kind whose YAML shape uses one cannot be expressed in HCL;
// the fix is a metadata change (an alias or a group), never a rename here.
var ReservedRootNames = []string{"connection", "count", "depends_on", "for_each", "lifecycle", "provider", "provisioner"}

// Attr is one attribute of a kind's Terraform schema, independent of the
// framework types: BuildSchema turns it into a schema.Attribute at runtime and
// cmd/generator renders it as Go source. One tree, so the two cannot differ.
type Attr struct {
	Name string
	// Type is string, int64, float64, bool, list, map, object (a group or a
	// timestamp) or list_object (a list of nested objects).
	Type string
	// ElemType is the element type of a list or map: string or int64.
	ElemType                                string
	Required, Optional, Computed, Sensitive bool
	Description                             string
	StringEnum                              []string
	IntEnum                                 []int64
	// Children are an object's or list_object's attributes, in YAML order.
	Children []*Attr
}

// node is one level of the YAML shape: a group, or the root.
type node struct {
	children map[string]*node
	order    []string
	leaves   map[string]*bmap.Field
}

func newNode() *node {
	return &node{children: map[string]*node{}, leaves: map[string]*bmap.Field{}}
}

// Attributes derives a kind's schema tree from its metadata (R1, R14):
//   - a group is an object (SingleNestedAttribute), a list of objects a
//     list_object (ListNestedAttribute: attribute syntax, never blocks);
//   - env and secret maps and free-form objects are maps of strings;
//   - a timestamp is the API's TimeResponse object, as in state mode;
//   - readOnly and displayReadOnly fields are Computed;
//   - a writable field is Required when required with no default, otherwise
//     Optional+Computed. A group with a Required member is Required.
//
// Defaults are not schema Defaults: the resolver applies them (R9) and the
// description says "Defaults to X".
func Attributes(k *bmap.Kind) ([]*Attr, error) {
	for _, name := range ReservedRootNames {
		for _, f := range k.Fields {
			if strings.Split(f.Path, ".")[0] == name {
				return nil, fmt.Errorf("%s: %q is reserved by Terraform at the top level of a resource (field %s)", k.Name, name, f.Name)
			}
		}
	}
	root := newNode()
	for _, f := range k.YAMLOrder() {
		segments := strings.Split(f.Path, ".")
		n := root
		for _, s := range segments[:len(segments)-1] {
			child, ok := n.children[s]
			if !ok {
				child = newNode()
				n.children[s] = child
				n.order = append(n.order, s)
			}
			n = child
		}
		leaf := segments[len(segments)-1]
		n.leaves[leaf] = f
		n.order = append(n.order, leaf)
	}
	attrs, _, err := nodeAttrs(k, root)
	return attrs, err
}

func nodeAttrs(k *bmap.Kind, n *node) ([]*Attr, bool, error) {
	var attrs []*Attr
	seen := map[string]bool{}
	anyRequired := false
	for _, name := range n.order {
		if seen[name] {
			continue
		}
		seen[name] = true
		if child, ok := n.children[name]; ok {
			children, required, err := nodeAttrs(k, child)
			if err != nil {
				return nil, false, fmt.Errorf("%s.%w", name, err)
			}
			attrs = append(attrs, &Attr{Name: name, Type: "object", Required: required, Optional: !required, Computed: !required, Children: children})
			anyRequired = anyRequired || required
			continue
		}
		a, err := leaf(k, name, n.leaves[name], !k.IsWritable(n.leaves[name]))
		if err != nil {
			return nil, false, fmt.Errorf("%s: %w", name, err)
		}
		attrs = append(attrs, a)
		anyRequired = anyRequired || a.Required
	}
	return attrs, anyRequired, nil
}

// fieldDefault is the field's spec default, at the kind level or on its
// descriptor (list items carry theirs on the descriptor).
func fieldDefault(k *bmap.Kind, f *bmap.Field) (any, bool) {
	if k != nil {
		if v, ok := k.Defaults.Get(f.Name); ok {
			return v, true
		}
	}
	if f.Desc.HasDefault {
		return f.Desc.Default, true
	}
	return nil, false
}

// describe is the attribute description: the spec's, plus its defaults and
// the type rules (R9, R19), so the docs say what a plan will check.
func describe(k *bmap.Kind, f *bmap.Field) string {
	text := strings.TrimSpace(f.Desc.Description)
	add := func(s string) {
		if text != "" && !strings.HasSuffix(text, ".") {
			text += "."
		}
		text = strings.TrimSpace(text + " " + s)
	}
	required := sortedTypes(f.Desc.TypeRequired)
	if len(required) > 0 {
		add("Required for " + strings.Join(required, " and ") + " containers.")
	}
	if f.Desc.TypeDefaults.Len() > 0 {
		for _, typ := range f.Desc.TypeDefaults.Keys() {
			v, _ := f.Desc.TypeDefaults.Get(typ)
			add("Defaults to `" + bmap.Encode(v) + "` for " + typ + " containers.")
		}
	} else if def, ok := fieldDefault(k, f); ok && def != nil && !isEmptyCollection(def) && len(required) == 0 {
		add("Defaults to `" + bmap.Encode(def) + "`.")
	}
	if forbidden := sortedTypes(f.Desc.TypeForbidden); len(forbidden) > 0 {
		add("Not allowed on " + strings.Join(forbidden, " or ") + " containers.")
	}
	return text
}

// sortedTypes lists a type set in a stable order.
func sortedTypes(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func isEmptyCollection(v any) bool {
	switch t := v.(type) {
	case []any:
		return len(t) == 0
	case *bmap.Map:
		return t.Len() == 0
	}
	return false
}

// leaf builds one field's attribute. k is nil inside list items.
func leaf(k *bmap.Kind, name string, f *bmap.Field, readOnly bool) (*Attr, error) {
	d := f.Desc
	_, defaulted := fieldDefault(k, f)
	a := &Attr{Name: name, Description: describe(k, f)}
	a.Required = !readOnly && d.Required && !defaulted
	a.Optional = !readOnly && !a.Required
	a.Computed = !a.Required
	// Masked says only that a read may return a placeholder (the merge keeps
	// prior state then); whether plans hide the value is x-sensitive's call.
	// Public certificates and config content are masked in list reads but are
	// not secret, so they stay visible (#2646).
	a.Sensitive = d.Sensitive || d.WriteOnly
	if d.Parent {
		// The URL parent: read-only in the body, but Terraform must be given it.
		a.Required, a.Optional, a.Computed = true, false, false
	}

	if f.Transform == "envVarMap" || f.Transform == "secretMap" || d.Freeform {
		a.Type, a.ElemType = "map", "string"
		return a, nil
	}
	switch d.Type {
	case "timestamp":
		// Timestamps are always read-only. With `format: date-time` they are
		// RFC 3339 strings in UTC (R20); a schema from before the rule keeps the
		// API's TimeResponse object, as its state-mode fixtures hold it.
		a.Required, a.Optional, a.Computed, a.Sensitive = false, false, true, false
		if d.Format == "date-time" {
			a.Type = "string"
			break
		}
		a.Type = "object"
		a.Children = []*Attr{
			{Name: "date", Type: "string", Computed: true},
			{Name: "timezonetype", Type: "int64", Computed: true},
			{Name: "timezone", Type: "string", Computed: true},
		}
	case "string":
		a.Type = "string"
		if !readOnly {
			for _, v := range d.Enum {
				if s, ok := v.(string); ok {
					a.StringEnum = append(a.StringEnum, s)
				}
			}
			sort.Strings(a.StringEnum)
		}
	case "integer":
		a.Type = "int64"
		if !readOnly {
			for _, v := range d.Enum {
				switch n := v.(type) {
				case int64:
					a.IntEnum = append(a.IntEnum, n)
				case float64:
					a.IntEnum = append(a.IntEnum, int64(n))
				}
			}
			sort.Slice(a.IntEnum, func(i, j int) bool { return a.IntEnum[i] < a.IntEnum[j] })
		}
	case "number":
		a.Type = "float64"
	case "boolean":
		a.Type = "bool"
	case "array":
		if len(f.Item) > 0 {
			a.Type, a.Sensitive = "list_object", false
			for _, item := range f.Item {
				child, err := leaf(nil, item.Path, item, readOnly || item.Desc.ReadOnly)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", item.Path, err)
				}
				a.Children = append(a.Children, child)
			}
			sort.Slice(a.Children, func(i, j int) bool { return a.Children[i].Name < a.Children[j].Name })
			return a, nil
		}
		a.Type = "list"
		switch d.ItemsType {
		case "string", "":
			a.ElemType = "string"
		case "integer":
			a.ElemType = "int64"
		default:
			return nil, fmt.Errorf("unsupported list of %s", d.ItemsType)
		}
	default:
		return nil, fmt.Errorf("unsupported type %s", d.Type)
	}
	return a, nil
}

// BuildSchema is the kind's framework schema.
func BuildSchema(k *bmap.Kind, description string) (schema.Schema, error) {
	attrs, err := Attributes(k)
	if err != nil {
		return schema.Schema{}, err
	}
	return schema.Schema{Version: SchemaVersion, Description: description, MarkdownDescription: description, Attributes: frameworkAttrs(attrs)}, nil
}

func frameworkAttrs(attrs []*Attr) map[string]schema.Attribute {
	out := make(map[string]schema.Attribute, len(attrs))
	for _, a := range attrs {
		out[a.Name] = a.framework()
	}
	return out
}

func (a *Attr) framework() schema.Attribute {
	switch a.Type {
	case "string":
		var v []validator.String
		if len(a.StringEnum) > 0 {
			v = []validator.String{stringvalidator.OneOf(a.StringEnum...)}
		}
		return schema.StringAttribute{Required: a.Required, Optional: a.Optional, Computed: a.Computed, Sensitive: a.Sensitive, Description: a.Description, MarkdownDescription: a.Description, Validators: v}
	case "int64":
		var v []validator.Int64
		if len(a.IntEnum) > 0 {
			v = []validator.Int64{int64validator.OneOf(a.IntEnum...)}
		}
		return schema.Int64Attribute{Required: a.Required, Optional: a.Optional, Computed: a.Computed, Sensitive: a.Sensitive, Description: a.Description, MarkdownDescription: a.Description, Validators: v}
	case "float64":
		return schema.Float64Attribute{Required: a.Required, Optional: a.Optional, Computed: a.Computed, Sensitive: a.Sensitive, Description: a.Description, MarkdownDescription: a.Description}
	case "bool":
		return schema.BoolAttribute{Required: a.Required, Optional: a.Optional, Computed: a.Computed, Sensitive: a.Sensitive, Description: a.Description, MarkdownDescription: a.Description}
	case "list":
		if a.ElemType == "int64" {
			return schema.ListAttribute{ElementType: types.Int64Type, Required: a.Required, Optional: a.Optional, Computed: a.Computed, Sensitive: a.Sensitive, Description: a.Description, MarkdownDescription: a.Description}
		}
		return schema.ListAttribute{ElementType: types.StringType, Required: a.Required, Optional: a.Optional, Computed: a.Computed, Sensitive: a.Sensitive, Description: a.Description, MarkdownDescription: a.Description}
	case "map":
		return schema.MapAttribute{ElementType: types.StringType, Required: a.Required, Optional: a.Optional, Computed: a.Computed, Sensitive: a.Sensitive, Description: a.Description, MarkdownDescription: a.Description}
	case "object":
		return schema.SingleNestedAttribute{Attributes: frameworkAttrs(a.Children), Required: a.Required, Optional: a.Optional, Computed: a.Computed, Description: a.Description, MarkdownDescription: a.Description}
	case "list_object":
		return schema.ListNestedAttribute{NestedObject: schema.NestedAttributeObject{Attributes: frameworkAttrs(a.Children)}, Required: a.Required, Optional: a.Optional, Computed: a.Computed, Description: a.Description, MarkdownDescription: a.Description}
	}
	panic("engine: unknown attribute type " + a.Type)
}
