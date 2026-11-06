// Package tfval converts between Terraform's tftypes.Value and the plain
// values internal/bmap works on: nil, bool, string, int64, float64, []any and
// *bmap.Map. An unknown value becomes the Unknown marker, at any depth.
package tfval

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/bmap"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type unknown struct{}

// Unknown marks a value Terraform does not know yet ("known after apply").
var Unknown any = unknown{}

// IsUnknown reports whether v is the Unknown marker.
func IsUnknown(v any) bool {
	_, ok := v.(unknown)
	return ok
}

// ToGo converts a Terraform value. Object and map keys come out in lexical
// order, which is also the order env maps are sent in (an accepted Terraform
// deviation).
func ToGo(v tftypes.Value) (any, error) {
	if !v.IsKnown() {
		return Unknown, nil
	}
	if v.IsNull() {
		return nil, nil
	}
	t := v.Type()
	switch {
	case t.Is(tftypes.String):
		var s string
		err := v.As(&s)
		return s, err
	case t.Is(tftypes.Bool):
		var b bool
		err := v.As(&b)
		return b, err
	case t.Is(tftypes.Number):
		f := new(big.Float)
		if err := v.As(&f); err != nil {
			return nil, err
		}
		if f.IsInt() {
			if i, acc := f.Int64(); acc == big.Exact {
				return i, nil
			}
		}
		out, _ := f.Float64()
		return out, nil
	case t.Is(tftypes.List{}), t.Is(tftypes.Set{}), t.Is(tftypes.Tuple{}):
		var elements []tftypes.Value
		if err := v.As(&elements); err != nil {
			return nil, err
		}
		list := make([]any, 0, len(elements))
		for _, e := range elements {
			g, err := ToGo(e)
			if err != nil {
				return nil, err
			}
			list = append(list, g)
		}
		return list, nil
	case t.Is(tftypes.Map{}), t.Is(tftypes.Object{}):
		var attrs map[string]tftypes.Value
		if err := v.As(&attrs); err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(attrs))
		for k := range attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		m := bmap.NewMap()
		for _, k := range keys {
			g, err := ToGo(attrs[k])
			if err != nil {
				return nil, err
			}
			m.Set(k, g)
		}
		return m, nil
	}
	return nil, fmt.Errorf("tfval: unsupported type %s", t)
}

// FromGo converts a plain value to a Terraform value of type t. An object
// attribute missing from the map is null. A key the object type does not
// declare is an error, so schema drift is caught rather than dropped.
func FromGo(v any, t tftypes.Type) (tftypes.Value, error) {
	if IsUnknown(v) {
		return tftypes.NewValue(t, tftypes.UnknownValue), nil
	}
	if v == nil {
		return tftypes.NewValue(t, nil), nil
	}
	switch {
	case t.Is(tftypes.String):
		s, ok := v.(string)
		if !ok {
			return tftypes.Value{}, fmt.Errorf("tfval: want string, got %T", v)
		}
		return tftypes.NewValue(t, s), nil
	case t.Is(tftypes.Bool):
		b, ok := v.(bool)
		if !ok {
			return tftypes.Value{}, fmt.Errorf("tfval: want bool, got %T", v)
		}
		return tftypes.NewValue(t, b), nil
	case t.Is(tftypes.Number):
		switch n := v.(type) {
		case int64:
			return tftypes.NewValue(t, new(big.Float).SetInt64(n)), nil
		case int:
			return tftypes.NewValue(t, new(big.Float).SetInt64(int64(n))), nil
		case float64:
			return tftypes.NewValue(t, big.NewFloat(n)), nil
		}
		return tftypes.Value{}, fmt.Errorf("tfval: want number, got %T", v)
	case t.Is(tftypes.List{}), t.Is(tftypes.Set{}):
		list, ok := v.([]any)
		if !ok {
			return tftypes.Value{}, fmt.Errorf("tfval: want list, got %T", v)
		}
		var elementType tftypes.Type
		if l, isList := t.(tftypes.List); isList {
			elementType = l.ElementType
		} else {
			elementType = t.(tftypes.Set).ElementType
		}
		elements := make([]tftypes.Value, 0, len(list))
		for i, e := range list {
			tv, err := FromGo(e, elementType)
			if err != nil {
				return tftypes.Value{}, fmt.Errorf("[%d]: %w", i, err)
			}
			elements = append(elements, tv)
		}
		return tftypes.NewValue(t, elements), nil
	case t.Is(tftypes.Map{}):
		m, ok := v.(*bmap.Map)
		if !ok {
			return tftypes.Value{}, fmt.Errorf("tfval: want map, got %T", v)
		}
		elementType := t.(tftypes.Map).ElementType
		values := map[string]tftypes.Value{}
		for _, k := range m.Keys() {
			item, _ := m.Get(k)
			tv, err := FromGo(item, elementType)
			if err != nil {
				return tftypes.Value{}, fmt.Errorf("%s: %w", k, err)
			}
			values[k] = tv
		}
		return tftypes.NewValue(t, values), nil
	case t.Is(tftypes.Object{}):
		m, ok := v.(*bmap.Map)
		if !ok {
			return tftypes.Value{}, fmt.Errorf("tfval: want object, got %T", v)
		}
		ot := t.(tftypes.Object)
		for _, k := range m.Keys() {
			if _, declared := ot.AttributeTypes[k]; !declared {
				return tftypes.Value{}, fmt.Errorf("tfval: attribute %q is not in the schema", k)
			}
		}
		values := map[string]tftypes.Value{}
		for name, at := range ot.AttributeTypes {
			item, _ := m.Get(name)
			tv, err := FromGo(item, at)
			if err != nil {
				return tftypes.Value{}, fmt.Errorf("%s: %w", name, err)
			}
			values[name] = tv
		}
		return tftypes.NewValue(t, values), nil
	}
	return tftypes.Value{}, fmt.Errorf("tfval: unsupported type %s", t)
}
