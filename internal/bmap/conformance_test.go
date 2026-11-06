package bmap

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Runs every conformance case in bahriya/openapi (RULES.md section 8), asserts
// the round trip on every toApi case, and writes build/conformance-report.json
// for `node <package>/conformance/check-report.mjs`.
//
// The package is read from BAHRIYA_OPENAPI_DIR, or from the monorepo checkout
// beside this repository. A missing package fails the test: a conformance run
// that silently runs nothing is not a pass.

type conformanceCase struct {
	ID      string
	Kind    string
	Type    string
	Options *Map
	Input   any
	Expect  any
	// Deprecations a toApi case must report (R1, fourth form); [] when absent.
	Deprecations []any
}

type reportEntry struct {
	Pass      bool  `json:"pass"`
	RoundTrip *bool `json:"roundTrip"`
}

func packageDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("BAHRIYA_OPENAPI_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "packages", "bahriya-openapi")
	}
	if _, err := os.Stat(filepath.Join(dir, "conformance", "RULES.md")); err != nil {
		t.Fatalf("bahriya/openapi not found at %s (set BAHRIYA_OPENAPI_DIR): %v", dir, err)
	}
	return dir
}

// decodeFixture turns {"$map": [[k, v], …]} into an ordered Map (RULES.md section 7).
func decodeFixture(v any) any {
	switch t := v.(type) {
	case *Map:
		if pairs, ok := mapGet(t, "$map").([]any); ok && t.Len() == 1 {
			m := NewMap()
			for _, p := range pairs {
				pair := p.([]any)
				m.Set(pair[0].(string), decodeFixture(pair[1]))
			}
			return m
		}
		out := NewMap()
		for _, k := range t.Keys() {
			item, _ := t.Get(k)
			out.Set(k, decodeFixture(item))
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = decodeFixture(t[i])
		}
		return out
	}
	return v
}

func loadCases(t *testing.T, dir string) []conformanceCase {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "conformance", "*", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no conformance fixtures under %s", dir)
	}
	sort.Strings(files)
	var cases []conformanceCase
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := Decode(data)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, item := range raw.([]any) {
			c := item.(*Map)
			cc := conformanceCase{
				Input:  decodeFixture(mapGet(c, "input")),
				Expect: decodeFixture(mapGet(c, "expect")),
			}
			cc.ID, _ = mapGet(c, "id").(string)
			cc.Kind, _ = mapGet(c, "kind").(string)
			cc.Type, _ = mapGet(c, "type").(string)
			cc.Options, _ = mapGet(c, "options").(*Map)
			cc.Deprecations, _ = mapGet(c, "deprecations").([]any)
			if cc.Deprecations == nil {
				cc.Deprecations = []any{}
			}
			cases = append(cases, cc)
		}
	}
	return cases
}

func TestEmbeddedSchemaMatchesPackage(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(packageDir(t), "specs", "resource-schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, resourceSchemaJSON) {
		t.Fatal("internal/bmap/resource-schema.json differs from the bahriya/openapi package: run `make generate`")
	}
}

func TestConformance(t *testing.T) {
	dir := packageDir(t)
	schemaData, err := os.ReadFile(filepath.Join(dir, "specs", "resource-schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := ParseSchema(schemaData)
	if err != nil {
		t.Fatal(err)
	}

	report := map[string]reportEntry{}
	for _, c := range loadCases(t, dir) {
		c := c
		t.Run(c.ID, func(t *testing.T) {
			entry := reportEntry{}
			defer func() { report[c.ID] = entry }()

			kind := schema.Kind(c.Kind)
			if kind == nil {
				t.Fatalf("unknown kind %s", c.Kind)
			}
			defaults := true
			if v, ok := mapGet(c.Options, "defaults").(bool); ok {
				defaults = v
			}
			input, _ := c.Input.(*Map)

			switch c.Type {
			case "toApi":
				got, deprecations, err := kind.ToAPIReport(input, defaults)
				if err != nil {
					t.Fatalf("toApi: %v", err)
				}
				if !JSONEquals(c.Expect, got) {
					t.Fatalf("\nexpected: %s\nactual:   %s", Encode(c.Expect), Encode(got))
				}
				gotDeprecations := make([]any, len(deprecations))
				for i, d := range deprecations {
					var use any = d.Use
					if d.Use == "" {
						use = nil
					}
					gotDeprecations[i] = MapOf("path", d.Path, "use", use)
				}
				if !OrderedEquals(c.Deprecations, gotDeprecations) && !(len(c.Deprecations) == 0 && len(gotDeprecations) == 0) {
					t.Fatalf("deprecations\nexpected: %s\nactual:   %s", Encode(c.Deprecations), Encode(gotDeprecations))
				}
				once, err := kind.ToAPI(input, true)
				if err != nil {
					t.Fatal(err)
				}
				twice, err := kind.ToAPI(kind.ToYAML(once, false), true)
				if err != nil {
					t.Fatal(err)
				}
				if !JSONEquals(once, twice) {
					t.Fatalf("round trip:\nonce:  %s\ntwice: %s", Encode(once), Encode(twice))
				}
				roundTrip := true
				entry.RoundTrip = &roundTrip
			case "toYaml", "toYamlState":
				got := kind.ToYAML(input, c.Type == "toYamlState")
				if !OrderedEquals(c.Expect, got) {
					t.Fatalf("\nexpected: %s\nactual:   %s", Encode(c.Expect), Encode(got))
				}
			case "applyDiff":
				current, _ := mapGet(input, "current").(*Map)
				doc, _ := mapGet(input, "doc").(*Map)
				payload, cleared, err := kind.ApplyDiff(current, doc)
				if err != nil {
					t.Fatal(err)
				}
				expect := c.Expect.(*Map)
				if !JSONEquals(mapGet(expect, "payload"), payload) {
					t.Fatalf("payload\nexpected: %s\nactual:   %s", Encode(mapGet(expect, "payload")), Encode(payload))
				}
				gotCleared := make([]any, len(cleared))
				for i, p := range cleared {
					gotCleared[i] = p
				}
				if !OrderedEquals(mapGet(expect, "cleared"), gotCleared) {
					t.Fatalf("cleared\nexpected: %s\nactual:   %s", Encode(mapGet(expect, "cleared")), Encode(gotCleared))
				}
			case "hcl":
				got := kind.ToHCL(input)
				if c.Expect != got {
					t.Fatalf("\nexpected:\n%s\nactual:\n%s", c.Expect, got)
				}
			case "error":
				_, err := kind.ToAPI(input, defaults)
				var mappingErr *MappingError
				if !errors.As(err, &mappingErr) {
					t.Fatalf("expected a mapping error, got %v", err)
				}
				if want, _ := mapGet(c.Expect.(*Map), "error").(string); mappingErr.Rule != want {
					t.Fatalf("expected %s, got %s", want, mappingErr.Rule)
				}
			default:
				t.Fatalf("unknown case type %s", c.Type)
			}
			entry.Pass = true
		})
	}

	writeReport(t, dir, report)
}

func writeReport(t *testing.T, dir string, cases map[string]reportEntry) {
	t.Helper()
	var pkg struct {
		Version string `json:"version"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		_ = json.Unmarshal(data, &pkg)
	}
	out, err := json.MarshalIndent(map[string]any{
		"package": pkg.Version,
		"runner":  "terraform-provider",
		"cases":   cases,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("..", "..", "build", "conformance-report.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
