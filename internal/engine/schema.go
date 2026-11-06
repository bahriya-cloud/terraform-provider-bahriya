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

// node is one level of the YAML shape: a group, or the root.
type node struct {
	children map[string]*node
	order    []string
	leaves   map[string]*bmap.Field
}

func newNode() *node {
	return &node{children: map[string]*node{}, leaves: map[string]*bmap.Field{}}
}

// BuildSchema derives a kind's Terraform schema from its metadata (R1, R14):
//   - a group is a SingleNestedAttribute, a list of objects a
//     ListNestedAttribute (attribute syntax, never blocks);
//   - env and secret maps and free-form objects are Map(String);
//   - readOnly and displayReadOnly fields are Computed;
//   - a writable field is Required when required with no default, otherwise
//     Optional+Computed.
//
// Defaults are not schema Defaults: the resolver applies them (R9) and the
// description says "Defaults to X". Implications and sensitive values are
// resolved there too.
func BuildSchema(k *bmap.Kind, description string) (schema.Schema, error) {
	for _, name := range ReservedRootNames {
		for _, f := range k.Fields {
			if strings.Split(f.Path, ".")[0] == name {
				return schema.Schema{}, fmt.Errorf("%s: %q is reserved by Terraform at the top level of a resource (field %s)", k.Name, name, f.Name)
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
	attrs, _, err := buildAttributes(k, root)
	if err != nil {
		return schema.Schema{}, err
	}
	return schema.Schema{Description: description, MarkdownDescription: description, Attributes: attrs}, nil
}

// buildAttributes returns the attributes of a node and whether any is Required.
// A group with a Required member is itself Required.
func buildAttributes(k *bmap.Kind, n *node) (map[string]schema.Attribute, bool, error) {
	attrs := map[string]schema.Attribute{}
	anyRequired := false
	for _, name := range n.order {
		if child, ok := n.children[name]; ok {
			if _, done := attrs[name]; done {
				continue
			}
			childAttrs, required, err := buildAttributes(k, child)
			if err != nil {
				return nil, false, fmt.Errorf("%s.%w", name, err)
			}
			attrs[name] = schema.SingleNestedAttribute{
				Attributes: childAttrs,
				Required:   required,
				Optional:   !required,
				Computed:   !required,
			}
			anyRequired = anyRequired || required
			continue
		}
		f := n.leaves[name]
		attr, required, err := leafAttribute(k, f, !k.IsWritable(f))
		if err != nil {
			return nil, false, fmt.Errorf("%s: %w", name, err)
		}
		attrs[name] = attr
		anyRequired = anyRequired || required
	}
	return attrs, anyRequired, nil
}

// hasDefault is true when the field has a spec default, at the kind level or
// on its descriptor (list items carry theirs on the descriptor).
func hasDefault(k *bmap.Kind, f *bmap.Field) (any, bool) {
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

// describe is the attribute description: the spec's, plus the default.
func describe(k *bmap.Kind, f *bmap.Field) string {
	text := strings.TrimSpace(f.Desc.Description)
	if def, ok := hasDefault(k, f); ok && def != nil && !isEmptyCollection(def) {
		if text != "" && !strings.HasSuffix(text, ".") {
			text += "."
		}
		text = strings.TrimSpace(text + " Defaults to `" + bmap.Encode(def) + "`.")
	}
	return text
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

// leafAttribute builds one field's attribute. k is nil inside list items,
// whose defaults sit on the descriptor.
func leafAttribute(k *bmap.Kind, f *bmap.Field, readOnly bool) (schema.Attribute, bool, error) {
	d := f.Desc
	_, defaulted := hasDefault(k, f)
	required := !readOnly && d.Required && !defaulted
	optional := !readOnly && !required
	computed := !required
	sensitive := d.Sensitive || d.WriteOnly || d.Masked != ""
	desc := describe(k, f)

	if f.Transform == "envVarMap" || f.Transform == "secretMap" || d.Freeform {
		return schema.MapAttribute{ElementType: types.StringType, Required: required, Optional: optional, Computed: computed, Sensitive: sensitive, Description: desc, MarkdownDescription: desc}, required, nil
	}
	switch d.Type {
	case "timestamp":
		// The API's TimeResponse, kept as-is in state mode (the state-mode
		// fixtures hold it as this object). Timestamps are always read-only.
		return schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"date":         schema.StringAttribute{Computed: true},
				"timezonetype": schema.Int64Attribute{Computed: true},
				"timezone":     schema.StringAttribute{Computed: true},
			},
			Computed: true, Description: desc, MarkdownDescription: desc,
		}, false, nil
	case "string":
		return schema.StringAttribute{Required: required, Optional: optional, Computed: computed, Sensitive: sensitive, Description: desc, MarkdownDescription: desc, Validators: stringEnum(d, readOnly)}, required, nil
	case "integer":
		return schema.Int64Attribute{Required: required, Optional: optional, Computed: computed, Sensitive: sensitive, Description: desc, MarkdownDescription: desc, Validators: intEnum(d, readOnly)}, required, nil
	case "number":
		return schema.Float64Attribute{Required: required, Optional: optional, Computed: computed, Sensitive: sensitive, Description: desc, MarkdownDescription: desc}, required, nil
	case "boolean":
		return schema.BoolAttribute{Required: required, Optional: optional, Computed: computed, Sensitive: sensitive, Description: desc, MarkdownDescription: desc}, required, nil
	case "array":
		if len(f.Item) > 0 {
			itemAttrs := map[string]schema.Attribute{}
			for _, item := range f.Item {
				a, _, err := leafAttribute(nil, item, readOnly || item.Desc.ReadOnly)
				if err != nil {
					return nil, false, fmt.Errorf("%s: %w", item.Path, err)
				}
				itemAttrs[item.Path] = a
			}
			return schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{Attributes: itemAttrs},
				Required:     required, Optional: optional, Computed: computed,
				Description: desc, MarkdownDescription: desc,
			}, required, nil
		}
		switch d.ItemsType {
		case "string", "":
			return schema.ListAttribute{ElementType: types.StringType, Required: required, Optional: optional, Computed: computed, Sensitive: sensitive, Description: desc, MarkdownDescription: desc}, required, nil
		case "integer":
			return schema.ListAttribute{ElementType: types.Int64Type, Required: required, Optional: optional, Computed: computed, Sensitive: sensitive, Description: desc, MarkdownDescription: desc}, required, nil
		}
	}
	return nil, false, fmt.Errorf("unsupported type %s (items %s)", d.Type, d.ItemsType)
}

// stringEnum validates a writable string enum (OneOf). Read-only fields are
// never validated: the API owns their values.
func stringEnum(d *bmap.Descriptor, readOnly bool) []validator.String {
	if readOnly || len(d.Enum) == 0 {
		return nil
	}
	var values []string
	for _, v := range d.Enum {
		if s, ok := v.(string); ok {
			values = append(values, s)
		}
	}
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	return []validator.String{stringvalidator.OneOf(values...)}
}

// intEnum validates a writable integer enum (OneOf).
func intEnum(d *bmap.Descriptor, readOnly bool) []validator.Int64 {
	if readOnly || len(d.Enum) == 0 {
		return nil
	}
	var values []int64
	for _, v := range d.Enum {
		switch n := v.(type) {
		case int64:
			values = append(values, n)
		case int:
			values = append(values, int64(n))
		case float64:
			values = append(values, int64(n))
		}
	}
	if len(values) == 0 {
		return nil
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return []validator.Int64{int64validator.OneOf(values...)}
}
