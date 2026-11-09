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
	"time"
	_ "time/tzdata" // R20 reads IANA zone names wherever the provider runs
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

// Violation records a field the resolved type does not allow, or requires and
// the document leaves out (R19).
type Violation struct {
	Path string
	Type string
	Rule string // "forbidden" or "required"
}

// ToAPI maps a YAML-shaped document to an API payload: R1-R8, then R9 when
// defaults is true.
func (k *Kind) ToAPI(doc *Map, defaults bool) (*Map, error) {
	out, _, _, err := k.ToAPIReport(doc, defaults)
	return out, err
}

// ToAPIReport is ToAPI, also returning the deprecations recorded by R1 and R3
// and the violations recorded by R19, each in field declaration order. The
// provider refuses a plan with violations.
func (k *Kind) ToAPIReport(doc *Map, defaults bool) (*Map, []Deprecation, []Violation, error) {
	out := NewMap()
	userSet := NewMap()
	deprecations := []Deprecation{}
	violations := []Violation{}
	typ := k.resolvedType(doc)

	// R19: type rules, checked on the document before defaults.
	if typ != "" {
		for _, f := range k.Fields {
			value, _ := k.read(doc, f)
			if f.Desc.TypeForbidden[typ] && value != absent && !isEmptyValue(value) {
				violations = append(violations, Violation{Path: f.Path, Type: typ, Rule: "forbidden"})
			}
			if f.Desc.TypeRequired[typ] && (value == absent || value == nil) {
				violations = append(violations, Violation{Path: f.Path, Type: typ, Rule: "required"})
			}
		}
	}

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
			return nil, nil, nil, err
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
				return nil, nil, nil, &MappingError{"R8", fmt.Sprintf("conflicting implications for '%s'", target)}
			}
			implied.Set(target, impliedValue)
		}
	}
	for _, target := range implied.Keys() {
		v, _ := implied.Get(target)
		out.Set(target, Clone(v))
	}

	// R9: absent writable fields take their effective default (a type default,
	// else the kind default). A field required for the type takes none; a field
	// forbidden for it takes only its kind default, an empty value, as the API
	// stores it.
	if defaults {
		for _, f := range k.Fields {
			if out.Has(f.Name) || !k.IsWritable(f) || f.Desc.TypeRequired[typ] {
				continue
			}
			var v any
			var ok bool
			if f.Desc.TypeForbidden[typ] {
				v, ok = k.Defaults.Get(f.Name)
			} else {
				v, ok = k.effectiveDefault(f, typ)
			}
			if ok {
				out.Set(f.Name, wire(Clone(v), f.Desc))
			}
		}
	}
	return out, deprecations, violations, nil
}

// resolvedType is the resolved type (RULES section 1) of a YAML document: the
// discriminator's value when set, else its default; "" on a kind without one.
func (k *Kind) resolvedType(doc *Map) string {
	f := k.byName[k.TypeDiscriminator]
	if f == nil {
		return ""
	}
	if v, _ := k.read(doc, f); v != absent {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	s, _ := f.Desc.Default.(string)
	return s
}

// payloadType is the resolved type of an API payload.
func (k *Kind) payloadType(api *Map) string {
	f := k.byName[k.TypeDiscriminator]
	if f == nil {
		return ""
	}
	if v, ok := api.Get(f.Name); ok {
		if s, isString := v.(string); isString && s != "" {
			return s
		}
	}
	s, _ := f.Desc.Default.(string)
	return s
}

// effectiveDefault is the type default for typ, else the kind default.
func (k *Kind) effectiveDefault(f *Field, typ string) (any, bool) {
	if typ != "" {
		if v, ok := f.Desc.TypeDefaults.Get(typ); ok {
			return v, true
		}
	}
	return k.Defaults.Get(f.Name)
}

// isJSONValue reports whether v is one of the values this package handles.
func isJSONValue(v any) bool {
	switch v.(type) {
	case nil, bool, string, int64, int, float64, []any, *Map:
		return true
	}
	return false
}

// isEmptyValue is R19: the values that count as not setting a forbidden field.
func isEmptyValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case bool:
		return !v
	case int64:
		return v == 0
	case int:
		return v == 0
	case float64:
		return v == 0
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case *Map:
		return v.Len() == 0
	}
	return false
}

// wire is R5 for a `wire: string` field: a whole number goes out as its
// base-10 text; a string passes unchanged.
func wire(value any, d *Descriptor) any {
	if d.Wire != "string" {
		return value
	}
	switch v := value.(type) {
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	case float64:
		if v == math.Trunc(v) {
			return strconv.FormatInt(int64(v), 10)
		}
	}
	return value
}

// timestamp is R20: RFC 3339 in UTC with whole seconds, from the API's
// TimeResponse object or an RFC 3339 / ISO 8601 string.
func timestamp(value any) (any, error) {
	var t time.Time
	switch v := value.(type) {
	case *Map:
		date, _ := mapGet(v, "date").(string)
		zone, _ := mapGet(v, "timezone").(string)
		if zone == "" {
			zone = "UTC"
		}
		loc, err := zoneLocation(zone)
		if err != nil {
			return nil, &MappingError{"R20", fmt.Sprintf("unknown time zone '%s'", zone)}
		}
		if len(date) < 19 {
			return nil, &MappingError{"R20", fmt.Sprintf("unreadable timestamp %q", date)}
		}
		t, err = time.ParseInLocation("2006-01-02 15:04:05", strings.Replace(date[:19], "T", " ", 1), loc)
		if err != nil {
			return nil, &MappingError{"R20", fmt.Sprintf("unreadable timestamp %q", date)}
		}
	case string:
		var err error
		if t, err = time.Parse(time.RFC3339Nano, v); err != nil {
			return nil, &MappingError{"R20", fmt.Sprintf("unreadable timestamp %q", v)}
		}
	default:
		return nil, &MappingError{"R20", fmt.Sprintf("expected a timestamp, got %v", value)}
	}
	return t.UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z"), nil
}

// zoneLocation reads an IANA zone name or a fixed offset such as +04:00.
func zoneLocation(zone string) (*time.Location, error) {
	if len(zone) == 6 && (zone[0] == '+' || zone[0] == '-') && zone[3] == ':' {
		hours, errH := strconv.Atoi(zone[1:3])
		minutes, errM := strconv.Atoi(zone[4:6])
		if errH != nil || errM != nil {
			return nil, fmt.Errorf("bad offset %s", zone)
		}
		seconds := hours*3600 + minutes*60
		if zone[0] == '-' {
			seconds = -seconds
		}
		return time.FixedZone(zone, seconds), nil
	}
	return time.LoadLocation(zone)
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

	if f.Desc.Wire == "string" && !isCollection(value) {
		return wire(value, f.Desc), nil // R5, a canonical integer carried as a string
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
					out.Set(f.Name, wire(Clone(f.Desc.Default), f.Desc))
				}
			}
		}
		mapped = append(mapped, out)
	}
	return mapped, nil
}

// ------------------------------------------------------------------ API -> YAML

// ToYAML maps an API payload to a YAML-shaped document: R11-R15. With state
// on (Terraform state, R15), every field is emitted and `kind` is omitted. It
// fails only on a value R12 or R20 cannot read.
func (k *Kind) ToYAML(api *Map, state bool) (*Map, error) {
	typ := k.payloadType(api)
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
			if s, ok := value.(string); ok && f.Desc.Masked != "" && s == f.Desc.Masked {
				continue // R11: the read placeholder is not a value; R16 keeps the material
			}
			if f.Desc.TypeForbidden[typ] && isEmptyValue(value) {
				continue // R11: what the API stores for a field the type does not allow
			}
		}
		if value == nil {
			emitted.Set(f.Name, nil)
			continue
		}
		reversed, err := reverse(value, f, state)
		if err != nil {
			return nil, err
		}
		if !state && !f.Desc.TypeRequired[typ] {
			if def, ok := k.effectiveDefault(f, typ); ok && JSONEquals(reversed, def) {
				continue // R11, compared after R12
			}
		}
		emitted.Set(f.Name, reversed)
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
	return out, nil
}

func reverse(value any, f *Field, state bool) (any, error) {
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
		return m, nil
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
				v, err := reverse(itemValue, itemField, state)
				if err != nil {
					return nil, err
				}
				mapped.Set(itemField.Path, v)
			}
			out = append(out, mapped)
		}
		return out, nil
	}
	// A value that is not JSON (a caller's marker, such as Terraform's unknown)
	// passes through: R12 and R20 read only what the API returns.
	if !isJSONValue(value) {
		return value, nil
	}
	if f.Desc.Wire == "string" {
		switch v := value.(type) {
		case int64:
			return v, nil
		case string:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				return n, nil // R12
			}
		}
		return nil, &MappingError{"R12", fmt.Sprintf("%s: expected a whole number, got %v", f.Path, value)}
	}
	if f.Desc.Type == "timestamp" { // R20
		return timestamp(value)
	}
	return Clone(value), nil
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

// Reset is a field an update would move from its current value to a default
// because the document does not state it (RULES.md R17).
type Reset struct {
	Path string
	From any
	To   any
}

// Resets lists, in R14 order, the writable fields the document does not state
// (ToAPI with defaults off leaves them out) whose current value the payload
// changes, other than those cleared already lists (RULES.md R17). Terraform
// shows these changes in its plan; the conformance suite checks the rule.
func (k *Kind) Resets(current, doc, payload *Map, cleared []string) ([]Reset, error) {
	stated, err := k.ToAPI(doc, false)
	if err != nil {
		return nil, err
	}
	isCleared := map[string]bool{}
	for _, p := range cleared {
		isCleared[p] = true
	}
	resets := []Reset{}
	for _, f := range k.YAMLOrder() {
		if !k.IsWritable(f) || isCleared[f.Path] || stated.Has(f.Name) {
			continue
		}
		was, ok := current.Get(f.Name)
		if !ok || was == nil {
			continue
		}
		now, ok := payload.Get(f.Name)
		if !ok {
			continue
		}
		if scalarText(was) != scalarText(now) {
			resets = append(resets, Reset{Path: f.Path, From: was, To: now})
		}
	}
	return resets, nil
}

// scalarText is a value as comparable text (R17), so the API's "3000" and a
// default of 3000 compare equal.
func scalarText(value any) string {
	switch v := value.(type) {
	case bool:
		return strconv.FormatBool(v)
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int, int64, json.Number:
		return fmt.Sprint(v)
	}
	b, _ := json.Marshal(value)
	return string(b)
}

// Unknown returns every key of doc that no R1 form reads (RULES.md R2), as
// paths in document order, list items included (init_jobs[0].mount_pth). A
// key naming a group (or a legacy path's prefix) holding an object is the
// group, so its keys are checked; the user keys of an R6 map are never
// unknown; `kind` is skipped at the top.
func (k *Kind) Unknown(doc *Map) []string {
	known := map[string]*Field{}
	groups := map[string]bool{}
	addPrefixes := func(path string, whole bool) {
		segments := strings.Split(path, ".")
		last := len(segments)
		if !whole {
			last--
		}
		for i := 1; i <= last; i++ {
			groups[strings.Join(segments[:i], ".")] = true
		}
	}
	for _, f := range k.Fields {
		forms := []string{f.Path}
		if f.Group != "" {
			forms = append(forms, f.Group+"."+f.Name, f.Name)
			addPrefixes(f.Group, true)
		} else {
			forms = append(forms, f.Name)
		}
		if f.LegacyPath != "" {
			forms = append(forms, f.LegacyPath)
			addPrefixes(f.LegacyPath, false) // `lifecycle` in lifecycle.init_jobs
		}
		for _, form := range forms {
			if _, ok := known[form]; !ok {
				known[form] = f
			}
		}
	}

	unknown := []string{}
	var walk func(node *Map, prefix string)
	walk = func(node *Map, prefix string) {
		for _, key := range node.Keys() {
			if prefix == "" && key == "kind" {
				continue
			}
			value, _ := node.Get(key)
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			if f, isField := known[path]; groups[path] && isMap(value) {
				if m, ok := value.(*Map); ok {
					walk(m, path)
				}
			} else if isField {
				unknownInValue(value, f, path, &unknown)
			} else {
				unknown = append(unknown, path)
			}
		}
	}
	walk(doc, "")
	return unknown
}

// unknownInValue is R2 inside a field's value: the items of a list mapped by
// R7, recursively. An R6 map holds user keys, and any other value is the
// field's own.
func unknownInValue(value any, f *Field, path string, unknown *[]string) {
	list, ok := value.([]any)
	if !f.HasItem || !ok {
		return
	}
	for i, element := range list {
		m, ok := element.(*Map)
		if !ok {
			continue
		}
		for _, key := range m.Keys() {
			inner, _ := m.Get(key)
			innerPath := fmt.Sprintf("%s[%d].%s", path, i, key)
			var match *Field
			for _, item := range f.Item {
				if item.Path == key || item.Name == key {
					match = item
					break
				}
			}
			if match == nil {
				*unknown = append(*unknown, innerPath)
				continue
			}
			unknownInValue(inner, match, innerPath, unknown)
		}
	}
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
	lines := []string{fmt.Sprintf("resource \"bahriya_%s\" %s {", k.TypeName(), Encode(label))}
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
