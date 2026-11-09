package bmap

import (
	"os"
	"path/filepath"
	"testing"
)

// FromState must invert ToYAML in state mode for every payload the fixtures
// hold: every toApi expectation, and every toYamlState input. Nil fields are
// compared as absent, because state mode emits nil for a field the payload
// lacks. Timestamps are read-only and kept in their RFC 3339 form (R20), so
// the payload's are compared in that form.
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
		state, err := kind.ToYAML(payload, true)
		if err != nil {
			t.Errorf("%s: %v", c.ID, err)
			continue
		}
		got, err := kind.FromState(state)
		if err != nil {
			t.Errorf("%s: %v", c.ID, err)
			continue
		}
		want := withoutNil(nullsAsDefaults(kind, payload))
		for _, f := range kind.Fields {
			if v, ok := want.Get(f.Name); ok && f.Desc.Type == "timestamp" {
				ts, err := timestamp(v)
				if err != nil {
					t.Fatalf("%s: %v", c.ID, err)
				}
				want.Set(f.Name, ts)
			}
		}
		if !JSONEquals(want, withoutNil(got)) {
			t.Errorf("%s:\nwant: %s\ngot:  %s", c.ID, Encode(want), Encode(withoutNil(got)))
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no payloads checked")
	}
}

// nullsAsDefaults is the payload as R21 reads it: a null in a field marked
// nullIsDefault (a list item's field included) is its default, so state round
// trips to that default rather than to null.
func nullsAsDefaults(kind *Kind, payload *Map) *Map {
	out := Clone(payload).(*Map)
	typ := kind.payloadType(payload)
	for _, f := range kind.Fields {
		v, ok := out.Get(f.Name)
		if !ok {
			continue
		}
		if v == nil && f.Desc.NullIsDefault {
			if def, has := kind.effectiveDefault(f, typ); has {
				out.Set(f.Name, Clone(def))
			}
			continue
		}
		list, isList := v.([]any)
		if !isList || len(f.Item) == 0 {
			continue
		}
		for _, element := range list {
			item, isMap := element.(*Map)
			if !isMap {
				continue
			}
			for _, itemField := range f.Item {
				if iv, present := item.Get(itemField.Name); present && iv == nil && itemField.Desc.NullIsDefault && itemField.Desc.HasDefault {
					item.Set(itemField.Name, Clone(itemField.Desc.Default))
				}
			}
		}
	}
	return out
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
