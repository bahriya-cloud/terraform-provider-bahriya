package tfval

import (
	"math/big"
	"strings"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/bmap"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var objType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"name":    tftypes.String,
	"enabled": tftypes.Bool,
	"size":    tftypes.Number,
	"ratio":   tftypes.Number,
	"regions": tftypes.List{ElementType: tftypes.String},
	"env":     tftypes.Map{ElementType: tftypes.String},
	"group": tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"ttl": tftypes.Number,
	}},
	"items": tftypes.List{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"username": tftypes.String,
		"password": tftypes.String,
	}}},
}}

func TestRoundTrip(t *testing.T) {
	in := bmap.MapOf(
		"name", "Shop API",
		"enabled", false,
		"size", int64(256),
		"ratio", 1.5,
		"regions", []any{"falkenstein-1"},
		"env", bmap.MapOf("B_VAR", "2", "A.DOTTED", "1"),
		"group", bmap.MapOf("ttl", Unknown),
		"items", []any{bmap.MapOf("username", "alice", "password", nil)},
	)
	tv, err := FromGo(in, objType)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ToGo(tv)
	if err != nil {
		t.Fatal(err)
	}
	m := out.(*bmap.Map)

	if v, _ := m.Get("size"); v != int64(256) {
		t.Errorf("an integral number must come back as int64, got %#v", v)
	}
	if v, _ := m.Get("ratio"); v != 1.5 {
		t.Errorf("a fractional number must come back as float64, got %#v", v)
	}
	group, _ := m.Get("group")
	if ttl, _ := group.(*bmap.Map).Get("ttl"); !IsUnknown(ttl) {
		t.Errorf("unknown must survive at depth, got %#v", ttl)
	}
	env, _ := m.Get("env")
	if keys := env.(*bmap.Map).Keys(); strings.Join(keys, ",") != "A.DOTTED,B_VAR" {
		t.Errorf("map keys must come out in lexical order, got %v", keys)
	}
	items, _ := m.Get("items")
	if pw, present := items.([]any)[0].(*bmap.Map).Get("password"); !present || pw != nil {
		t.Errorf("a null attribute must be present as nil, got %#v (present %v)", pw, present)
	}
	in.Set("group", bmap.MapOf("ttl", Unknown))
	if !bmap.JSONEquals(withoutUnknown(in), withoutUnknown(m)) {
		t.Errorf("round trip changed the value:\nin:  %s\nout: %s", bmap.Encode(withoutUnknown(in)), bmap.Encode(withoutUnknown(m)))
	}
}

func TestMissingAttributeIsNull(t *testing.T) {
	tv, err := FromGo(bmap.MapOf("name", "x"), objType)
	if err != nil {
		t.Fatal(err)
	}
	var attrs map[string]tftypes.Value
	if err := tv.As(&attrs); err != nil {
		t.Fatal(err)
	}
	if !attrs["size"].IsNull() || !attrs["group"].IsNull() {
		t.Error("attributes absent from the map must be null")
	}
}

func TestUndeclaredAttributeIsAnError(t *testing.T) {
	_, err := FromGo(bmap.MapOf("name", "x", "surprise", true), objType)
	if err == nil || !strings.Contains(err.Error(), "surprise") {
		t.Fatalf("an attribute the schema does not declare must be an error, got %v", err)
	}
}

func TestTypeMismatchIsAnError(t *testing.T) {
	if _, err := FromGo(bmap.MapOf("size", "256"), objType); err == nil {
		t.Fatal("a string for a number attribute must be an error, not a silent cast")
	}
}

func TestWholeValueUnknownAndNull(t *testing.T) {
	unknown, _ := FromGo(Unknown, objType)
	if unknown.IsKnown() {
		t.Error("Unknown must become an unknown value")
	}
	if g, _ := ToGo(unknown); !IsUnknown(g) {
		t.Error("an unknown value must become Unknown")
	}
	null, _ := FromGo(nil, objType)
	if !null.IsNull() {
		t.Error("nil must become null")
	}
	if g, _ := ToGo(null); g != nil {
		t.Error("null must become nil")
	}
}

func TestLargeIntegerStaysExact(t *testing.T) {
	big1 := new(big.Float).SetInt64(9007199254740993) // 2^53 + 1, not exact as float64
	tv := tftypes.NewValue(tftypes.Number, big1)
	g, err := ToGo(tv)
	if err != nil {
		t.Fatal(err)
	}
	if g != int64(9007199254740993) {
		t.Errorf("integers beyond float64 precision must stay exact, got %#v", g)
	}
}

func withoutUnknown(v any) any {
	switch t := v.(type) {
	case *bmap.Map:
		out := bmap.NewMap()
		for _, k := range t.Keys() {
			item, _ := t.Get(k)
			if !IsUnknown(item) {
				out.Set(k, withoutUnknown(item))
			}
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = withoutUnknown(t[i])
		}
		return out
	}
	return v
}
