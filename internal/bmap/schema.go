package bmap

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"
)

// resourceSchemaJSON is bahriya/openapi's specs/resource-schema.json, copied
// here by `make generate` and checked by `make verify`. It is the only source
// of mapping facts (CLAUDE.md, "YAML and Terraform aliases").
//
//go:embed resource-schema.json
var resourceSchemaJSON []byte

// Schema is every kind's metadata.
type Schema struct {
	kinds map[string]*Kind
}

// Kind is one kind's entry of resource-schema.json.
type Kind struct {
	Name            string
	Fields          []*Field // declaration order (the order of `properties`)
	GroupOrder      []string
	Defaults        *Map
	byName          map[string]*Field
	topGroups       map[string]bool
	readOnly        map[string]bool
	displayReadOnly map[string]bool
}

// Field is one property with its alias entry, at the top level or inside a
// list item.
type Field struct {
	Name      string // API name
	Path      string // alias path, or the API name
	Group     string
	Transform string // envVarMap, secretMap or ""
	Implies   *Map   // target API field => implied value
	// LegacyPath is a top-level field's path before a rename, still read
	// for one Reis minor release (R1, fourth form).
	LegacyPath string
	Desc       *Descriptor
	// HasItem is true when the alias entry has `item`, so list elements are
	// mapped against Item (R7).
	HasItem bool
	Item    []*Field // descriptor items.properties, with the entry's item aliases
}

// Descriptor is a property descriptor (RULES.md section 1).
type Descriptor struct {
	Type            string
	Nullable        bool
	Required        bool
	Default         any
	HasDefault      bool
	Enum            []any
	ReadOnly        bool
	WriteOnly       bool
	CreateOnly      bool
	DisplayReadOnly bool
	Sensitive       bool
	Masked          string
	Volatile        bool
	Reference       string
	ItemKey         string
	Freeform        bool
	// InputDeprecated marks a read-only field clients once wrote, to no
	// effect; setting it records a deprecation (R3).
	InputDeprecated bool
	// RotateOnly marks versioned material that only POST …/rotate changes;
	// an update ignores it (x-rotate-only).
	RotateOnly bool
	// Immutable marks a field fixed at creation that neither an update nor a
	// rotation changes (x-immutable).
	Immutable   bool
	Description string
	ItemsType   string
}

var embedded *Schema

// Embedded returns the schema compiled into the provider.
func Embedded() *Schema {
	if embedded == nil {
		s, err := ParseSchema(resourceSchemaJSON)
		if err != nil {
			panic(fmt.Sprintf("bmap: embedded resource-schema.json: %v", err))
		}
		embedded = s
	}
	return embedded
}

// ParseSchema reads resource-schema.json.
func ParseSchema(data []byte) (*Schema, error) {
	root, err := Decode(data)
	if err != nil {
		return nil, err
	}
	top, ok := root.(*Map)
	if !ok {
		return nil, fmt.Errorf("bmap: resource-schema.json is not an object")
	}
	s := &Schema{kinds: map[string]*Kind{}}
	for _, name := range top.Keys() {
		raw, _ := top.Get(name)
		meta, ok := raw.(*Map)
		if !ok {
			return nil, fmt.Errorf("bmap: kind %s is not an object", name)
		}
		kind, err := parseKind(name, meta)
		if err != nil {
			return nil, fmt.Errorf("bmap: kind %s: %w", name, err)
		}
		s.kinds[name] = kind
	}
	return s, nil
}

// Kind returns a kind's metadata, or nil.
func (s *Schema) Kind(name string) *Kind {
	return s.kinds[name]
}

// Kinds lists the kind names, sorted.
func (s *Schema) Kinds() []string {
	names := make([]string, 0, len(s.kinds))
	for n := range s.kinds {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func parseKind(name string, meta *Map) (*Kind, error) {
	k := &Kind{
		Name:            name,
		byName:          map[string]*Field{},
		topGroups:       map[string]bool{},
		readOnly:        stringSet(mapGet(meta, "readOnly")),
		displayReadOnly: stringSet(mapGet(meta, "displayReadOnly")),
		Defaults:        asMap(mapGet(meta, "defaults")),
	}
	for _, g := range asList(mapGet(meta, "groupOrder")) {
		group, _ := g.(string)
		k.GroupOrder = append(k.GroupOrder, group)
		k.topGroups[strings.SplitN(group, ".", 2)[0]] = true
	}

	aliases := asMap(mapGet(meta, "aliases"))
	properties := asMap(mapGet(meta, "properties"))
	if properties.Len() == 0 {
		return nil, fmt.Errorf("no properties")
	}
	fields, err := buildFields(properties, aliases)
	if err != nil {
		return nil, err
	}
	k.Fields = fields
	for _, f := range fields {
		k.byName[f.Name] = f
	}
	return k, nil
}

// buildFields pairs each property with the alias entry naming it.
func buildFields(properties, aliases *Map) ([]*Field, error) {
	entryByField := map[string]*Map{}
	pathByField := map[string]string{}
	for _, path := range aliases.Keys() {
		raw, _ := aliases.Get(path)
		entry := asMap(raw)
		field, _ := mapGet(entry, "field").(string)
		if field == "" {
			return nil, fmt.Errorf("alias %s has no field", path)
		}
		entryByField[field] = entry
		pathByField[field] = path
	}

	var fields []*Field
	for _, name := range properties.Keys() {
		raw, _ := properties.Get(name)
		desc := parseDescriptor(asMap(raw))
		f := &Field{Name: name, Path: name, Desc: desc}
		entry := entryByField[name]
		if entry != nil {
			f.Path = pathByField[name]
			f.Group, _ = mapGet(entry, "group").(string)
			f.Transform, _ = mapGet(entry, "transform").(string)
			f.LegacyPath, _ = mapGet(entry, "legacyPath").(string)
			if implies, ok := mapGet(entry, "implies").(*Map); ok {
				f.Implies = implies
			}
			_, f.HasItem = entry.Get("item")
		}
		itemProps := asMap(mapGet(asMap(mapGet(asMap(raw), "items")), "properties"))
		if itemProps.Len() > 0 {
			item, err := buildFields(itemProps, asMap(mapGet(entry, "item")))
			if err != nil {
				return nil, fmt.Errorf("%s items: %w", name, err)
			}
			f.Item = item
		}
		fields = append(fields, f)
	}
	return fields, nil
}

func parseDescriptor(m *Map) *Descriptor {
	d := &Descriptor{}
	d.Type, _ = mapGet(m, "type").(string)
	d.Nullable, _ = mapGet(m, "nullable").(bool)
	d.Required, _ = mapGet(m, "required").(bool)
	d.Default, d.HasDefault = m.Get("default")
	d.Enum = asList(mapGet(m, "enum"))
	d.ReadOnly, _ = mapGet(m, "readOnly").(bool)
	d.WriteOnly, _ = mapGet(m, "writeOnly").(bool)
	d.CreateOnly, _ = mapGet(m, "createOnly").(bool)
	d.DisplayReadOnly, _ = mapGet(m, "displayReadOnly").(bool)
	d.Sensitive, _ = mapGet(m, "sensitive").(bool)
	d.Masked, _ = mapGet(m, "masked").(string)
	d.Volatile, _ = mapGet(m, "volatile").(bool)
	d.Reference, _ = mapGet(m, "reference").(string)
	d.ItemKey, _ = mapGet(m, "itemKey").(string)
	d.Freeform, _ = mapGet(m, "freeform").(bool)
	d.InputDeprecated, _ = mapGet(m, "inputDeprecated").(bool)
	d.RotateOnly, _ = mapGet(m, "rotateOnly").(bool)
	d.Immutable, _ = mapGet(m, "immutable").(bool)
	d.Description, _ = mapGet(m, "description").(string)
	d.ItemsType, _ = mapGet(asMap(mapGet(m, "items")), "type").(string)
	return d
}

// Field returns the field with that API name, or nil.
func (k *Kind) Field(name string) *Field {
	return k.byName[name]
}

// IsWritable is true when the field is neither readOnly nor displayReadOnly.
func (k *Kind) IsWritable(f *Field) bool {
	return !f.Desc.ReadOnly && !f.Desc.DisplayReadOnly && !k.readOnly[f.Name] && !k.displayReadOnly[f.Name]
}

// IsDisplayReadOnly is true for fields shown in YAML but never sent.
func (k *Kind) IsDisplayReadOnly(f *Field) bool {
	return k.displayReadOnly[f.Name] || f.Desc.DisplayReadOnly
}

// isStrippedReadOnly is R11's read-only test: readOnly and not displayReadOnly.
func (k *Kind) isStrippedReadOnly(f *Field) bool {
	return k.readOnly[f.Name] || (f.Desc.ReadOnly && !f.Desc.DisplayReadOnly)
}

// YAMLOrder lists the fields in R14 order: ungrouped in declaration order,
// then each top-level group in groupOrder.
func (k *Kind) YAMLOrder() []*Field {
	var order []*Field
	for _, f := range k.Fields {
		if f.Group == "" {
			order = append(order, f)
		}
	}
	for _, top := range k.GroupOrder {
		for _, f := range k.Fields {
			if f.Group != "" && (f.Group == top || strings.HasPrefix(f.Group, top+".")) {
				order = append(order, f)
			}
		}
	}
	return order
}

// ------------------------------------------------------------------ helpers

func mapGet(m *Map, key string) any {
	v, _ := m.Get(key)
	return v
}

func asMap(v any) *Map {
	if m, ok := v.(*Map); ok {
		return m
	}
	return NewMap()
}

func asList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}

func stringSet(v any) map[string]bool {
	set := map[string]bool{}
	for _, item := range asList(v) {
		if s, ok := item.(string); ok {
			set[s] = true
		}
	}
	return set
}
