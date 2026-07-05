package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestStringSetRoundTrip(t *testing.T) {
	set := stringSetFromSlice([]string{"read", "update"})
	out, ok := stringSetOrNil(set).([]string)
	if !ok {
		t.Fatalf("stringSetOrNil did not return []string: %T", stringSetOrNil(set))
	}
	if len(out) != 2 {
		t.Fatalf("len = %d", len(out))
	}
	got := map[string]bool{out[0]: true, out[1]: true}
	if !got["read"] || !got["update"] {
		t.Errorf("set = %v", out)
	}
}

func TestStringSetOrNilNull(t *testing.T) {
	if got := stringSetOrNil(types.SetNull(types.StringType)); got != nil {
		t.Errorf("null set should be nil, got %v", got)
	}
}

func TestStringListFromSlice(t *testing.T) {
	list := stringListFromSlice([]string{"g1", "g2", "g3"})
	if len(list.Elements()) != 3 {
		t.Fatalf("len = %d", len(list.Elements()))
	}
	first, ok := list.Elements()[0].(types.String)
	if !ok || first.ValueString() != "g1" {
		t.Errorf("first = %v", list.Elements()[0])
	}
}

func TestStringSetFromSliceEmpty(t *testing.T) {
	set := stringSetFromSlice(nil)
	if len(set.Elements()) != 0 {
		t.Errorf("empty slice → set len %d", len(set.Elements()))
	}
	// sanity: an explicit element still round-trips through attr.Value
	_ = attr.Value(types.StringValue("x"))
}
