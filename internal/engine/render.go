package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// RenderAttributes renders a schema tree as a Go map[string]schema.Attribute
// literal, for cmd/generator. It is the source form of frameworkAttrs: the
// generated-schema drift test compares the two.
func RenderAttributes(attrs []*Attr, indent int) string {
	var b strings.Builder
	pad := strings.Repeat("\t", indent)
	b.WriteString("map[string]schema.Attribute{\n")
	for _, a := range attrs {
		fmt.Fprintf(&b, "%s\t%q: %s,\n", pad, a.Name, renderAttr(a, indent+1))
	}
	b.WriteString(pad + "}")
	return b.String()
}

// Imports lists the packages a rendered tree needs beyond schema and types.
func Imports(attrs []*Attr) (stringEnum, intEnum bool) {
	for _, a := range attrs {
		if len(a.StringEnum) > 0 {
			stringEnum = true
		}
		if len(a.IntEnum) > 0 {
			intEnum = true
		}
		s, i := Imports(a.Children)
		stringEnum, intEnum = stringEnum || s, intEnum || i
	}
	return stringEnum, intEnum
}

func renderAttr(a *Attr, indent int) string {
	var fields []string
	add := func(format string, args ...any) { fields = append(fields, fmt.Sprintf(format, args...)) }
	flags := func() {
		if a.Required {
			add("Required: true")
		}
		if a.Optional {
			add("Optional: true")
		}
		if a.Computed {
			add("Computed: true")
		}
	}
	description := func() {
		if a.Description != "" {
			add("Description: %s", strconv.Quote(a.Description))
			add("MarkdownDescription: %s", strconv.Quote(a.Description))
		}
	}
	sensitive := func() {
		if a.Sensitive {
			add("Sensitive: true")
		}
	}

	var typ string
	switch a.Type {
	case "string":
		typ = "schema.StringAttribute"
		flags()
		sensitive()
		description()
		if len(a.StringEnum) > 0 {
			quoted := make([]string, len(a.StringEnum))
			for i, s := range a.StringEnum {
				quoted[i] = strconv.Quote(s)
			}
			add("Validators: []validator.String{stringvalidator.OneOf(%s)}", strings.Join(quoted, ", "))
		}
	case "int64":
		typ = "schema.Int64Attribute"
		flags()
		sensitive()
		description()
		if len(a.IntEnum) > 0 {
			nums := make([]string, len(a.IntEnum))
			for i, n := range a.IntEnum {
				nums[i] = strconv.FormatInt(n, 10)
			}
			add("Validators: []validator.Int64{int64validator.OneOf(%s)}", strings.Join(nums, ", "))
		}
	case "float64":
		typ = "schema.Float64Attribute"
		flags()
		sensitive()
		description()
	case "bool":
		typ = "schema.BoolAttribute"
		flags()
		sensitive()
		description()
	case "list":
		typ = "schema.ListAttribute"
		if a.ElemType == "int64" {
			add("ElementType: types.Int64Type")
		} else {
			add("ElementType: types.StringType")
		}
		flags()
		sensitive()
		description()
	case "map":
		typ = "schema.MapAttribute"
		add("ElementType: types.StringType")
		flags()
		sensitive()
		description()
	case "object":
		typ = "schema.SingleNestedAttribute"
		add("Attributes: %s", RenderAttributes(a.Children, indent))
		flags()
		description()
	case "list_object":
		typ = "schema.ListNestedAttribute"
		add("NestedObject: schema.NestedAttributeObject{Attributes: %s}", RenderAttributes(a.Children, indent))
		flags()
		description()
	default:
		panic("engine: unknown attribute type " + a.Type)
	}
	pad := strings.Repeat("\t", indent)
	return typ + "{\n" + pad + "\t" + strings.Join(fields, ",\n"+pad+"\t") + ",\n" + pad + "}"
}
