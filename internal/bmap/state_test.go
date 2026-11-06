package bmap

import (
	"os"
	"path/filepath"
	"testing"
)

// FromState must invert ToYAML in state mode for every payload the fixtures
// hold: every toApi expectation, and every toYamlState input. Nil fields are
// compared as absent, because state mode emits nil for a field the payload
// lacks.
func TestFromStateInvertsStateMode(t *testing.T) {
	dir := packageDir(t)
	data, err := os.ReadFile(filepath.Join(dir, "specs", "resource-schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := ParseSchema(data)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, c := range loadCases(t, dir) {
		var payload *Map
		switch c.Type {
		case "toApi":
			payload, _ = c.Expect.(*Map)
		case "toYamlState":
			payload, _ = c.Input.(*Map)
		default:
			continue
		}
		kind := schema.Kind(c.Kind)
		got, err := kind.FromState(kind.ToYAML(payload, true))
		if err != nil {
			t.Errorf("%s: %v", c.ID, err)
			continue
		}
		if !JSONEquals(withoutNil(payload), withoutNil(got)) {
			t.Errorf("%s:\nwant: %s\ngot:  %s", c.ID, Encode(withoutNil(payload)), Encode(withoutNil(got)))
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no payloads checked")
	}
}

func withoutNil(m *Map) *Map {
	out := NewMap()
	for _, k := range m.Keys() {
		if v, _ := m.Get(k); v != nil {
			out.Set(k, v)
		}
	}
	return out
}
