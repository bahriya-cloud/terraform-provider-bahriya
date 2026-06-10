package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

var _ = yaml.Marshal

type openAPIDoc struct {
	Components struct {
		Schemas map[string]rawSchema `yaml:"schemas"`
	} `yaml:"components"`
}

type rawSchema struct {
	Required   []string               `yaml:"required"`
	Properties map[string]rawProperty `yaml:"properties"`
}

type rawProperty struct {
	Type        any          `yaml:"type"`
	Items       *rawProperty `yaml:"items,omitempty"`
	Ref         string       `yaml:"$ref,omitempty"`
	ReadOnly    bool         `yaml:"readOnly,omitempty"`
	Description string       `yaml:"description,omitempty"`
	Default     any          `yaml:"default,omitempty"`
	Extensions  map[string]any
}

func (p *rawProperty) UnmarshalYAML(node *yaml.Node) error {
	type alias rawProperty
	var raw alias
	if err := node.Decode(&raw); err != nil {
		return err
	}
	*p = rawProperty(raw)
	if node.Kind == yaml.MappingNode {
		p.Extensions = map[string]any{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if strings.HasPrefix(key, "x-") {
				var val any
				if err := node.Content[i+1].Decode(&val); err == nil {
					p.Extensions[key] = val
				}
			}
		}
	}
	return nil
}

func (p rawProperty) isCreateOnly() bool {
	if v, ok := p.Extensions["x-createOnly"]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// resolveRef extracts the schema name from a $ref like "#/components/schemas/Hostname".
func resolveRef(ref string) string {
	const prefix = "#/components/schemas/"
	if strings.HasPrefix(ref, prefix) {
		return ref[len(prefix):]
	}
	return ""
}

func parseResource(specPath string, descriptor resourceDescriptor) (*Resource, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", specPath, err)
	}
	var doc openAPIDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", specPath, err)
	}
	rs, ok := doc.Components.Schemas[descriptor.SchemaName]
	if !ok {
		return nil, fmt.Errorf("%s: schema %q not found in spec", specPath, descriptor.SchemaName)
	}

	required := map[string]bool{}
	for _, r := range rs.Required {
		required[r] = true
	}

	sensitiveSet := map[string]bool{}
	for _, f := range descriptor.SensitiveFields {
		sensitiveSet[f] = true
	}

	skipSet := map[string]bool{}
	for _, f := range descriptor.SkipFields {
		skipSet[f] = true
	}

	attrs := make([]Attribute, 0, len(rs.Properties))
	for apiName, prop := range rs.Properties {
		if skipSet[apiName] {
			continue
		}
		typ, nested := resolveAttrType(prop, doc.Components.Schemas)
		if typ == "" {
			continue
		}
		attr := Attribute{
			TfName:      snakeCase(apiName),
			APIName:     apiName,
			GoName:      pascalCase(apiName),
			Type:        typ,
			Description: strings.TrimSpace(prop.Description),
			Sensitive:   sensitiveSet[apiName],
			Nested:      nested,
		}

		switch {
		case prop.ReadOnly:
			attr.Computed = true
		case required[apiName] && prop.Default == nil:
			attr.Required = true
		default:
			attr.Optional = true
		}

		if apiName == "handle" {
			attr.RequiresReplace = true
		}
		if prop.isCreateOnly() {
			attr.RequiresReplace = true
		}

		attrs = append(attrs, attr)
	}
	sort.Slice(attrs, func(i, j int) bool {
		bandI := attrBand(attrs[i])
		bandJ := attrBand(attrs[j])
		if bandI != bandJ {
			return bandI < bandJ
		}
		return attrs[i].TfName < attrs[j].TfName
	})

	return &Resource{
		Name:             descriptor.Name,
		GoName:           descriptor.GoName,
		HasStatus:        descriptor.HasStatus,
		ReadyStatus:      descriptor.ReadyStatus,
		TerminatedStatus: descriptor.TerminatedStatus,
		URLBase:          descriptor.URLBase,
		URLItem:          descriptor.URLItem,
		HandleURL:        descriptor.HandleURL,
		HandlePathName:   descriptor.HandlePathName,
		CollectionKey:    descriptor.CollectionKey,
		DeleteMethod:     descriptor.DeleteMethod,
		DeleteURL:        descriptor.DeleteURL,
		ConfirmDelete:    descriptor.ConfirmDelete,
		Attributes:       attrs,
	}, nil
}

// resolveAttrType determines the Go/TF type for a property. For $ref array
// items, it resolves the referenced schema into NestedFields. Returns the type
// string and (for nested lists) the inner fields.
func resolveAttrType(p rawProperty, schemas map[string]rawSchema) (string, []NestedField) {
	t := primitiveType(p.Type)
	switch t {
	case "string":
		return "string", nil
	case "integer":
		return "int64", nil
	case "boolean":
		return "bool", nil
	case "array":
		if p.Items == nil {
			return "", nil
		}
		// Plain array of primitives.
		if p.Items.Ref == "" {
			inner := primitiveType(p.Items.Type)
			switch inner {
			case "string":
				return "list_string", nil
			case "integer":
				return "list_integer", nil
			}
			return "", nil
		}
		// $ref array — resolve the nested schema.
		refName := resolveRef(p.Items.Ref)
		if refName == "" {
			return "", nil
		}
		nested, ok := schemas[refName]
		if !ok {
			return "", nil
		}
		fields := parseNestedFields(nested)
		if len(fields) == 0 {
			return "", nil
		}
		return "list_nested", fields
	}
	return "", nil
}

func parseNestedFields(schema rawSchema) []NestedField {
	required := map[string]bool{}
	for _, r := range schema.Required {
		required[r] = true
	}
	var fields []NestedField
	for name, prop := range schema.Properties {
		ft := primitiveType(prop.Type)
		var goType string
		switch ft {
		case "string":
			goType = "string"
		case "integer":
			goType = "int64"
		case "boolean":
			goType = "bool"
		case "array":
			if prop.Items != nil {
				inner := primitiveType(prop.Items.Type)
				if inner == "string" {
					goType = "list_string"
				}
			}
		}
		if goType == "" {
			continue
		}
		fields = append(fields, NestedField{
			TfName:   snakeCase(name),
			APIName:  name,
			GoName:   pascalCase(name),
			Type:     goType,
			Required: required[name],
		})
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].TfName < fields[j].TfName
	})
	return fields
}

func goType(p rawProperty) (string, bool) {
	t := primitiveType(p.Type)
	switch t {
	case "string":
		return "string", true
	case "integer":
		return "int64", true
	case "boolean":
		return "bool", true
	case "array":
		if p.Items == nil {
			return "", false
		}
		if p.Items.Ref != "" {
			return "", false
		}
		inner := primitiveType(p.Items.Type)
		if inner == "string" {
			return "list_string", true
		}
		return "", false
	}
	return "", false
}

func primitiveType(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && s != "null" {
				return s
			}
		}
	}
	return ""
}

func attrBand(a Attribute) int {
	if a.TfName == "id" {
		return 0
	}
	if a.TfName == "handle" {
		return 1
	}
	if a.Required {
		return 2
	}
	if a.Optional {
		return 3
	}
	return 4
}

func pascalCase(s string) string {
	parts := strings.Split(snakeCase(s), "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if up, ok := initialisms[strings.ToLower(p)]; ok {
			b.WriteString(up)
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r + 32)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var initialisms = map[string]string{
	"id":   "ID",
	"cpu":  "CPU",
	"url":  "URL",
	"uri":  "URI",
	"api":  "API",
	"http": "HTTP",
	"dns":  "DNS",
	"tls":  "TLS",
	"ssh":  "SSH",
	"ip":   "IP",
	"mb":   "MB",
	"gb":   "GB",
	"ttl":  "TTL",
}

type resourceDescriptor struct {
	Name             string
	GoName           string
	SchemaName       string
	URLBase          string
	URLItem          string
	HandleURL        string
	HandlePathName   string
	CollectionKey    string
	DeleteMethod     string
	DeleteURL        string
	ConfirmDelete    bool
	HasStatus        bool
	ReadyStatus      string
	TerminatedStatus string
	SensitiveFields  []string
	SkipFields       []string
}

var descriptors = map[string]resourceDescriptor{
	"project": {
		Name:           "project",
		GoName:         "Project",
		SchemaName:     "Project",
		URLBase:        "/organisations/%s/projects",
		URLItem:        "/organisations/%s/projects/%s",
		HandleURL:      "/organisations/%s/project/%s",
		HandlePathName: "project",
		CollectionKey:  "projects",
		ConfirmDelete:  true,
	},
	"registry": {
		Name:            "registry",
		GoName:          "Registry",
		SchemaName:      "Registry",
		URLBase:         "/organisations/%s/registries",
		URLItem:         "/organisations/%s/registries/%s",
		HandleURL:       "/organisations/%s/registry/%s",
		HandlePathName:  "registry",
		SensitiveFields: []string{"password"},
	},
	"secret": {
		Name:            "secret",
		GoName:          "Secret",
		SchemaName:      "Secret",
		URLBase:         "/organisations/%s/secrets",
		URLItem:         "/organisations/%s/secrets/%s",
		HandleURL:       "/organisations/%s/secret/%s",
		HandlePathName:  "secret",
		SensitiveFields: []string{"value"},
	},
	"memcached": {
		Name:             "memcached",
		GoName:           "Memcached",
		SchemaName:       "Memcached",
		URLBase:          "/organisations/%s/memcached",
		URLItem:          "/organisations/%s/memcached/%s",
		HandleURL:        "/organisations/%s/memcache/%s",
		HandlePathName:   "memcache",
		HasStatus:        true,
		ReadyStatus:      "running",
		TerminatedStatus: "terminated",
	},
	"container": {
		Name:             "container",
		GoName:           "Container",
		SchemaName:       "Container",
		URLBase:          "/organisations/%s/containers",
		URLItem:          "/organisations/%s/containers/%s",
		HandleURL:        "/organisations/%s/container/%s",
		HandlePathName:   "container",
		DeleteMethod:     "POST",
		DeleteURL:        "/organisations/%s/containers/%s/terminate",
		HasStatus:        true,
		ReadyStatus:      "running",
		TerminatedStatus: "terminated",
	},
}
