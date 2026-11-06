package bmap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Values handled by this package are JSON values: nil, bool, string, int64,
// float64, []any and *Map. Every JSON object is an ordered *Map, because the
// rules depend on order in three places: env and secret maps keep insertion
// order (R6), toYaml output is in declaration order (R14), and the metadata's
// properties are listed in declaration order.

// Map is a string-keyed map that keeps insertion order.
type Map struct {
	keys   []string
	values map[string]any
}

// NewMap returns an empty Map.
func NewMap() *Map {
	return &Map{values: map[string]any{}}
}

// MapOf builds a Map from alternating keys and values, in order.
func MapOf(pairs ...any) *Map {
	m := NewMap()
	for i := 0; i+1 < len(pairs); i += 2 {
		m.Set(pairs[i].(string), pairs[i+1])
	}
	return m
}

// Len is the number of entries.
func (m *Map) Len() int {
	if m == nil {
		return 0
	}
	return len(m.keys)
}

// Keys returns the keys in insertion order.
func (m *Map) Keys() []string {
	if m == nil {
		return nil
	}
	return append([]string(nil), m.keys...)
}

// Get returns the value at key, and whether the key is present.
func (m *Map) Get(key string) (any, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m.values[key]
	return v, ok
}

// Has reports whether key is present, even with a nil value.
func (m *Map) Has(key string) bool {
	_, ok := m.Get(key)
	return ok
}

// Set adds or replaces key. A new key goes last; a replaced key keeps its place.
func (m *Map) Set(key string, value any) {
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Delete removes key.
func (m *Map) Delete(key string) {
	if _, ok := m.values[key]; !ok {
		return
	}
	delete(m.values, key)
	for i, k := range m.keys {
		if k == key {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			break
		}
	}
}

// MarshalJSON writes the entries in order.
func (m *Map) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range m.Keys() {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		value, err := json.Marshal(m.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// Decode parses JSON into ordered values. An integer literal becomes int64, any
// other number float64.
func Decode(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("bmap: trailing data after JSON value")
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			m := NewMap()
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyTok.(string)
				if !ok {
					return nil, fmt.Errorf("bmap: object key %v is not a string", keyTok)
				}
				value, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				m.Set(key, value)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return m, nil
		case '[':
			list := []any{}
			for dec.More() {
				value, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				list = append(list, value)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return list, nil
		}
		return nil, fmt.Errorf("bmap: unexpected delimiter %v", t)
	case json.Number:
		return number(t)
	default:
		return t, nil // nil, bool or string
	}
}

func number(n json.Number) (any, error) {
	s := n.String()
	if !strings.ContainsAny(s, ".eE") {
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i, nil
		}
	}
	return n.Float64()
}

// Encode writes v as JSON without HTML escaping, keeping map order.
func Encode(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Sprintf("%v", v)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// ------------------------------------------------------------------ equality

// JSONEquals is JSON deep equality (RULES.md section 1): numbers by value,
// maps without regard to key order, lists in order. As in the PHP and
// JavaScript ports, an empty list equals an empty map.
func JSONEquals(a, b any) bool {
	return equal(a, b, false)
}

// OrderedEquals is JSONEquals, except that map keys must also be in the same
// order. toYaml, toYamlState and hcl output is compared this way (RULES.md
// section 7).
func OrderedEquals(a, b any) bool {
	return equal(a, b, true)
}

func equal(a, b any, ordered bool) bool {
	if isEmptyCollection(a) && isEmptyCollection(b) {
		return true
	}
	if fa, ok := toFloat(a); ok {
		fb, ok := toFloat(b)
		return ok && fa == fb
	}
	switch av := a.(type) {
	case nil:
		return b == nil
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !equal(av[i], bv[i], ordered) {
				return false
			}
		}
		return true
	case *Map:
		bv, ok := b.(*Map)
		if !ok || av.Len() != bv.Len() {
			return false
		}
		for i, k := range av.keys {
			if ordered && bv.keys[i] != k {
				return false
			}
			other, present := bv.Get(k)
			if !present || !equal(av.values[k], other, ordered) {
				return false
			}
		}
		return true
	}
	return false
}

func isEmptyCollection(v any) bool {
	switch t := v.(type) {
	case []any:
		return len(t) == 0
	case *Map:
		return t.Len() == 0
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int64:
		return float64(t), true
	case int:
		return float64(t), true
	case float64:
		return t, true
	}
	return 0, false
}

// Clone deep-copies a value.
func Clone(v any) any {
	switch t := v.(type) {
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = Clone(t[i])
		}
		return out
	case *Map:
		out := NewMap()
		for _, k := range t.keys {
			out.Set(k, Clone(t.values[k]))
		}
		return out
	}
	return v
}
