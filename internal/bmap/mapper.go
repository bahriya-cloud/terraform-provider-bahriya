// Package bmap is the canonical resource mapping, a port of bahriya/openapi's
// conformance/RULES.md (ADR 2026-11-04, Canonical resource mapping rules and
// conformance suite). Rule numbers in comments refer to that document. The
// conformance fixtures beside it are the contract, run by conformance_test.go.
// The PHP port in reis/src/Schema/CanonicalMapper.php is the reference this one
// mirrors.
package bmap

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// MappingError is a toApi failure, citing the rule that failed.
type MappingError struct {
	Rule    string
	Message string
}

func (e *MappingError) Error() string {
	return e.Rule + ": " + e.Message
}

type missing struct{}

// absent marks a key that is not in the document, as opposed to a nil value.
var absent = missing{}

// ------------------------------------------------------------------ YAML -> API

// Deprecation records a value read at a legacy path (R1, fourth form), or an
// input-deprecated read-only field the document sets (R3).
type Deprecation struct {
	Path string // the legacy path, or the input-deprecated field's path
	Use  string // the current path; "" (null) means remove it
}

// ToAPI maps a YAML-shaped document to an API payload: R1-R8, then R9 when
// defaults is true.
func (k *Kind) ToAPI(doc *Map, defaults bool) (*Map, error) {
	out, _, err := k.ToAPIReport(doc, defaults)
	return out, err
}

// ToAPIReport is ToAPI, also returning the deprecations recorded by R1, in
// field declaration order.
func (k *Kind) ToAPIReport(doc *Map, defaults bool) (*Map, []Deprecation, error) {
	out := NewMap()
	userSet := NewMap()
	deprecations := []Deprecation{}

	for _, f := range k.Fields {
		value, legacy := k.read(doc, f)
		if legacy && value != absent {
			deprecations = append(deprecations, Deprecation{Path: f.LegacyPath, Use: f.Path})
		}
		if f.Desc.InputDeprecated && value != absent && value != nil {
			deprecations = append(deprecations, Deprecation{Path: f.Path}) // R3: remove it
		}
		if value == absent || !k.IsWritable(f) || value == nil {
			continue // R1 absent, R3 read-only, R4 null
		}
		mapped, err := mapValue(value, f, defaults)
		if err != nil {
			return nil, nil, err
		}
		out.Set(f.Name, mapped)
		userSet.Set(f.Name, value)
	}

	// R8: implications from set values, order-independent, explicit wins.
	implied := NewMap()
	for _, name := range userSet.Keys() {
		f := k.byName[name]
		value, _ := userSet.Get(name)
		if f.Implies == nil || !IsSet(value) {
			continue
		}
		for _, target := range f.Implies.Keys() {
			impliedValue, _ := f.Implies.Get(target)
			if userSet.Has(target) {
				continue
			}
			if previous, ok := implied.Get(target); ok && !JSONEquals(previous, impliedValue) {
				return nil, nil, &MappingError{"R8", fmt.Sprintf("conflicting implications for '%s'", target)}
			}
			implied.Set(target, impliedValue)
		}
	}
	for _, target := range implied.Keys() {
		v, _ := implied.Get(target)
		out.Set(target, Clone(v))
	}

	// R9: defaults fill absent writable fields.
	if defaults {
		for _, name := range k.Defaults.Keys() {
			f := k.byName[name]
			if f == nil || out.Has(name) || !k.IsWritable(f) {
				continue
			}
			v, _ := k.Defaults.Get(name)
			out.Set(name, Clone(v))
		}
	}
	return out, deprecations, nil
}

// read is R1: the field's path, then its API-name path, then (grouped fields)
// its API name at the top level, then its legacy path. A group key holding an
// object is the group. The bool is true when the value came from the legacy path.
func (k *Kind) read(doc *Map, f *Field) (any, bool) {
	value := get(doc, f.Path)
	if value == absent {
		if f.Group != "" {
			value = get(doc, f.Group+"."+f.Name)
		} else {
			value = get(doc, f.Name)
		}
	}
	if value == absent && f.Group != "" {
		value = get(doc, f.Name)
		if value != absent && k.topGroups[f.Name] && isMap(value) {
			value = absent
		}
	}
	if value == absent && f.LegacyPath != "" {
		if legacy := get(doc, f.LegacyPath); legacy != absent {
			return legacy, true
		}
	}
	return value, false
}

func mapValue(value any, f *Field, defaults bool) (any, error) {
	if f.Transform == "envVarMap" || f.Transform == "secretMap" {
		if list, ok := value.([]any); ok {
			if len(list) == 0 {
				return []any{}, nil // R6: an empty map becomes []
			}
			return mapItems(list, f.Item, defaults) // already the API shape
		}
		m, ok := value.(*Map)
		if !ok {
			return value, nil
		}
		list := []any{}
		for _, key := range m.Keys() {
			item, _ := m.Get(key)
			if key == "" || strings.Contains(key, "=") {
				return nil, &MappingError{"R6", fmt.Sprintf("invalid environment key '%s'", key)}
			}
			if item == nil {
				continue
			}
			if f.Transform == "envVarMap" {
				list = append(list, MapOf("key", key, "value", Cast(item)))
			} else {
				list = append(list, MapOf("secret", item, "name", key))
			}
		}
		return list, nil
	}

	if list, ok := value.([]any); ok && f.HasItem {
		return mapItems(list, f.Item, defaults)
	}

	if f.Desc.Type == "string" && !isCollection(value) {
		return Cast(value), nil
	}
	return Clone(value), nil
}

// mapItems is R7: each object element is mapped against the item fields.
func mapItems(list []any, itemFields []*Field, defaults bool) ([]any, error) {
	mapped := make([]any, 0, len(list))
	for _, element := range list {
		m, ok := element.(*Map)
		if !ok {
			if l, isList := element.([]any); isList && len(l) == 0 {
				m = NewMap() // an empty element is an empty object, as in PHP
			} else {
				mapped = append(mapped, Clone(element))
				continue
			}
		}
		out := NewMap()
		for _, f := range itemFields {
			var value any = absent
			if v, present := m.Get(f.Path); present {
				value = v
			} else if v, present := m.Get(f.Name); present {
				value = v
			}
			if value == absent || value == nil || f.Desc.ReadOnly || f.Desc.DisplayReadOnly {
				continue
			}
			v, err := mapValue(value, f, defaults)
			if err != nil {
				return nil, err
			}
			out.Set(f.Name, v)
		}
		if defaults {
			for _, f := range itemFields {
				if !out.Has(f.Name) && f.Desc.HasDefault && !f.Desc.ReadOnly {
					out.Set(f.Name, Clone(f.Desc.Default))
				}
			}
		}
		mapped = append(mapped, out)
	}
	return mapped, nil
}

// ------------------------------------------------------------------ API -> YAML

// ToYAML maps an API payload to a YAML-shaped document: R11-R15. With state
// on (Terraform state, R15), every field is emitted and `kind` is omitted.
func (k *Kind) ToYAML(api *Map, state bool) *Map {
	emitted := NewMap()
	for _, f := range k.Fields {
		value, present := api.Get(f.Name)
		if !present {
			if state {
				emitted.Set(f.Name, nil)
			}
			continue
		}
		if !state {
			if k.isStrippedReadOnly(f) || value == nil {
				continue // R11, R12
			}
			if def, ok := k.Defaults.Get(f.Name); ok && JSONEquals(value, def) {
				continue // R11
			}
		}
		if value == nil {
			emitted.Set(f.Name, nil)
			continue
		}
		emitted.Set(f.Name, reverse(value, f, state))
	}

	// R13: emit an implied field when the API value would not survive a re-apply.
	if !state {
		for changed := true; changed; {
			changed = false
			for _, name := range emitted.Keys() {
				f := k.byName[name]
				value, _ := emitted.Get(name)
				if f.Implies == nil || !IsSet(value) {
					continue
				}
				for _, target := range f.Implies.Keys() {
					if emitted.Has(target) {
						continue
					}
					impliedValue, _ := f.Implies.Get(target)
					apiValue, _ := api.Get(target)
					if apiValue != nil && !JSONEquals(apiValue, impliedValue) {
						emitted.Set(target, Clone(apiValue))
						changed = true
					}
				}
			}
		}
	}

	// R14: kind, ungrouped fields in declaration order, then groups in groupOrder.
	out := NewMap()
	if !state {
		out.Set("kind", k.Name)
	}
	for _, f := range k.YAMLOrder() {
		if v, ok := emitted.Get(f.Name); ok {
			setPath(out, f.Path, v)
		}
	}
	return out
}

func reverse(value any, f *Field, state bool) any {
	list, isList := value.([]any)
	if isList && (f.Transform == "envVarMap" || f.Transform == "secretMap") {
		m := NewMap()
		for _, element := range list {
			item, ok := element.(*Map)
			if !ok {
				continue
			}
			if f.Transform == "envVarMap" {
				key, _ := mapGet(item, "key").(string)
				m.Set(key, Clone(mapGet(item, "value")))
			} else {
				name, _ := mapGet(item, "name").(string)
				m.Set(name, Clone(mapGet(item, "secret")))
			}
		}
		return m
	}
	if isList && f.HasItem {
		out := make([]any, 0, len(list))
		for _, element := range list {
			item, ok := element.(*Map)
			if !ok {
				out = append(out, Clone(element))
				continue
			}
			mapped := NewMap()
			for _, itemField := range f.Item {
				itemValue, present := item.Get(itemField.Name)
				if !present {
					continue
				}
				readOnly := itemField.Desc.ReadOnly && !itemField.Desc.DisplayReadOnly
				if !state && (itemValue == nil || readOnly) {
					continue
				}
				if itemValue == nil {
					mapped.Set(itemField.Path, nil)
					continue
				}
				mapped.Set(itemField.Path, reverse(itemValue, itemField, state))
			}
			out = append(out, mapped)
		}
		return out
	}
	return Clone(value)
}

// ------------------------------------------------------------------ apply

// ApplyDiff is R16 (omitted means default) and R17 (report what an apply would
// clear). It returns the update payload and the YAML paths it would clear.
func (k *Kind) ApplyDiff(current, doc *Map) (*Map, []string, error) {
	payload := NewMap()
	for _, name := range current.Keys() {
		if f := k.byName[name]; f != nil && k.IsWritable(f) {
			v, _ := current.Get(name)
			payload.Set(name, Clone(v))
		}
	}
	mapped, err := k.ToAPI(doc, true)
	if err != nil {
		return nil, nil, err
	}
	for _, name := range mapped.Keys() {
		v, _ := mapped.Get(name)
		payload.Set(name, v)
	}

	cleared := []string{}
	for _, f := range k.YAMLOrder() {
		if !k.IsWritable(f) || (f.Desc.Type != "array" && f.Desc.Type != "object") {
			continue
		}
		was, _ := current.Get(f.Name)
		if !isCollection(was) || isEmptyCollection(was) {
			continue
		}
		now, _ := payload.Get(f.Name)
		if now == nil || isEmptyCollection(now) {
			cleared = append(cleared, f.Path)
		}
	}
	return payload, cleared, nil
}

// ------------------------------------------------------------------ HCL

// ToHCL renders a YAML-shaped document as canonical HCL (RULES.md section 6).
func (k *Kind) ToHCL(doc *Map) string {
	doc = Clone(doc).(*Map)
	for _, f := range k.Fields {
		if k.IsDisplayReadOnly(f) {
			unsetPath(doc, f.Path)
		}
	}
	quoted := map[string]bool{}
	for _, f := range k.Fields {
		if f.Desc.Freeform || f.Transform == "envVarMap" || f.Transform == "secretMap" {
			quoted[f.Path] = true
		}
		// Maps inside list items (an init job's env) are maps too: quoted keys,
		// and `{}` when empty, at every depth.
		if f.HasItem {
			itemMapPaths(f.Path+"[]", f.Item, quoted)
		}
	}

	label := "this" // a kind without a handle (billing_details) is labelled "this"
	if k.byName["handle"] != nil {
		label, _ = mapGet(doc, "handle").(string)
	}
	lines := []string{fmt.Sprintf("resource \"bahriya_%s\" %s {", k.Name, Encode(label))}
	for _, key := range doc.Keys() {
		value, _ := doc.Get(key)
		if key == "kind" || value == nil {
			continue
		}
		lines = append(lines, "  "+key+" = "+hclValue(value, 1, key, quoted))
	}
	lines = append(lines, "}")
	return strings.Join(lines, "\n") + "\n"
}

func itemMapPaths(prefix string, itemFields []*Field, quoted map[string]bool) {
	for _, f := range itemFields {
		if f.Transform == "envVarMap" || f.Transform == "secretMap" || f.Desc.Freeform {
			quoted[prefix+"."+f.Path] = true
		} else if f.HasItem {
			itemMapPaths(prefix+"."+f.Path+"[]", f.Item, quoted)
		}
	}
}

func hclValue(value any, indent int, path string, quoted map[string]bool) string {
	pad := strings.Repeat("  ", indent)
	switch v := value.(type) {
	case bool:
		return strconv.FormatBool(v)
	case int64, int, float64:
		return Encode(v)
	case string:
		return Encode(v)
	case []any:
		if len(v) == 0 {
			return emptyCollection(path, quoted)
		}
		allScalar := true
		for _, element := range v {
			if isCollection(element) {
				allScalar = false
				break
			}
		}
		if allScalar {
			parts := make([]string, len(v))
			for i, element := range v {
				parts[i] = hclValue(element, indent, path, quoted)
			}
			return "[" + strings.Join(parts, ", ") + "]"
		}
		lines := []string{"["}
		for _, element := range v {
			lines = append(lines, pad+"  "+hclValue(element, indent+1, path+"[]", quoted)+",")
		}
		lines = append(lines, pad+"]")
		return strings.Join(lines, "\n")
	case *Map:
		if v.Len() == 0 {
			return emptyCollection(path, quoted)
		}
		isQuoted := quoted[path]
		lines := []string{"{"}
		for _, key := range v.Keys() {
			item, _ := v.Get(key)
			if item == nil {
				continue
			}
			name := key
			if isQuoted {
				name = Encode(key)
			}
			lines = append(lines, pad+"  "+name+" = "+hclValue(item, indent+1, path+"."+key, quoted))
		}
		lines = append(lines, pad+"}")
		return strings.Join(lines, "\n")
	}
	return "null"
}

// emptyCollection decides `{}` or `[]` from the field, as the PHP port must,
// because PHP cannot tell an empty list from an empty map.
func emptyCollection(path string, quoted map[string]bool) string {
	if quoted[path] {
		return "{}"
	}
	return "[]"
}

// ------------------------------------------------------------------ helpers

// IsSet is R8's test: not nil, not "", not an empty list or map. false and 0
// are set.
func IsSet(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case *Map:
		return v.Len() > 0
	}
	return true
}

// Cast is R5: JSON semantics for scalars; strings and collections unchanged.
func Cast(value any) any {
	switch v := value.(type) {
	case bool:
		return strconv.FormatBool(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	case float64:
		// Shortest round-trip text; an integral float has no fraction ("2", not "2.0").
		if v == math.Trunc(v) && math.Abs(v) < 1e15 {
			return strconv.FormatInt(int64(v), 10)
		}
		b, err := json.Marshal(v)
		if err != nil {
			return strconv.FormatFloat(v, 'g', -1, 64)
		}
		return string(b)
	}
	return value
}

func isCollection(v any) bool {
	switch v.(type) {
	case []any, *Map:
		return true
	}
	return false
}

// isMap mirrors PHP's test: an object, or an empty list (PHP cannot tell them apart).
func isMap(v any) bool {
	switch t := v.(type) {
	case *Map:
		return true
	case []any:
		return len(t) == 0
	}
	return false
}

func get(doc *Map, path string) any {
	var current any = doc
	for _, segment := range strings.Split(path, ".") {
		m, ok := current.(*Map)
		if !ok {
			return absent
		}
		v, present := m.Get(segment)
		if !present {
			return absent
		}
		current = v
	}
	return current
}

func setPath(doc *Map, path string, value any) {
	segments := strings.Split(path, ".")
	current := doc
	for _, segment := range segments[:len(segments)-1] {
		next, ok := mapGet(current, segment).(*Map)
		if !ok {
			next = NewMap()
			current.Set(segment, next)
		}
		current = next
	}
	current.Set(segments[len(segments)-1], value)
}

// unsetPath removes a path, then any group the removal left empty.
func unsetPath(doc *Map, path string) {
	head, rest, nested := strings.Cut(path, ".")
	if !doc.Has(head) {
		return
	}
	if !nested {
		doc.Delete(head)
		return
	}
	child, ok := mapGet(doc, head).(*Map)
	if !ok {
		return
	}
	unsetPath(child, rest)
	if child.Len() == 0 {
		doc.Delete(head)
	}
}
