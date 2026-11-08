package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/bmap"
	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/tfval"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// testdata/v<version>-resource-schemas.json is `terraform providers schema
// -json` of that released binary, resource schemas only (jq -S): the v0 state
// shapes the upgrader reads directly, frozen, never regenerated. 0.3.x and
// 0.4.x upgrade directly; a schema diff of all nine 0.3.x tags against 0.4.2
// found no attribute whose name stayed while its type changed, and one that
// was removed (container healthcheckscheme, in v0Drops).
var frozenSchemas = map[string]string{
	"0.4.2": "053d0d61ed97827f7461bd95898adbe52d48493366dbd88ab77b9c1868250b43",
	"0.3.8": "6218267c52090f7ea683b16b2afe66d25fd12fc9943fd2f0cee72c8e2862e1f3",
}

func v042Attributes(t *testing.T) map[string][]string {
	t.Helper()
	return frozenAttributes(t, "0.4.2")
}

func frozenAttributes(t *testing.T, version string) map[string][]string {
	t.Helper()
	file := "v" + version + "-resource-schemas.json"
	data, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != frozenSchemas[version] {
		t.Fatalf("testdata/%s changed: it is a frozen v0 state shape and must not be regenerated", file)
	}
	var schemas map[string]struct {
		Block struct {
			Attributes map[string]json.RawMessage `json:"attributes"`
		} `json:"block"`
	}
	if err := json.Unmarshal(data, &schemas); err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for typeName, s := range schemas {
		for a := range s.Block.Attributes {
			out[typeName] = append(out[typeName], a)
		}
	}
	return out
}

func engineResource(t *testing.T, kind string) *Resource {
	t.Helper()
	r := New(kind)().(*Resource)
	return r
}

// upgrade runs the 0 → 1 upgrader on raw v0 state JSON.
func upgrade(t *testing.T, r *Resource, v0 *bmap.Map) (resource.UpgradeStateResponse, *bmap.Map) {
	t.Helper()
	resp := resource.UpgradeStateResponse{State: tfsdk.State{Schema: r.schema, Raw: tftypes.NewValue(r.objectType(context.Background()), nil)}}
	r.UpgradeState(context.Background())[0].StateUpgrader(context.Background(),
		resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: []byte(bmap.Encode(v0))}}, &resp)
	if resp.Diagnostics.HasError() {
		return resp, nil
	}
	got, err := tfval.ToGo(resp.State.Raw)
	if err != nil {
		t.Fatal(err)
	}
	return resp, got.(*bmap.Map)
}

// Every v0.4.2 and v0.3.8 attribute of an engine kind is a v1 field name, a
// rename or a deliberate drop; nothing else could ever reach the "cannot map"
// error from a 0.3.x or 0.4.x state.
func TestEveryV042AttributeIsMapped(t *testing.T) {
	for version := range frozenSchemas {
		attrs := frozenAttributes(t, version)
		checkMapped(t, version, attrs)
	}
}

func checkMapped(t *testing.T, version string, attrs map[string][]string) {
	for name := range Descriptors {
		k := bmap.Embedded().Kind(name)
		for _, a := range attrs["bahriya_"+k.TypeName()] {
			_, renamed := v0Renames[name][a]
			_, dropped := v0Drops[name][a]
			if k.Field(a) == nil && !renamed && !dropped {
				t.Errorf("bahriya_%s.%s is in the v%s schema but neither a v1 field, a rename nor a drop", k.TypeName(), a, version)
			}
		}
	}
}

// A v0 state upgrades to the v1 state the engine would write for the same API
// record, for every kind with a state-mode conformance case: the record is
// restricted to the attributes v0.4.2 actually had.
func TestV0StatesUpgradeToTheV1Shape(t *testing.T) {
	for version := range frozenSchemas {
		t.Run("v"+version, func(t *testing.T) { upgradeShape(t, frozenAttributes(t, version)) })
	}
}

func upgradeShape(t *testing.T, attrs map[string][]string) {
	upgraded := 0
	for _, c := range stateCases(t) {
		if _, existed := attrs["bahriya_"+bmap.Embedded().Kind(c.kind).TypeName()]; !existed {
			continue // the kind is newer than this release (mysql in 0.3.8)
		}
		r := engineResource(t, c.kind)
		k := r.kind
		had := map[string]bool{}
		for _, a := range attrs["bahriya_"+k.TypeName()] {
			had[a] = true
		}
		v0, record := bmap.NewMap(), bmap.NewMap()
		for _, key := range c.input.Keys() {
			v, _ := c.input.Get(key)
			stateName := key
			for from, to := range v0Renames[c.kind] {
				if to == key {
					stateName = from
				}
			}
			if had[stateName] {
				v0.Set(stateName, v)
				record.Set(key, v)
			}
		}
		resp, got := upgrade(t, r, v0)
		if got == nil {
			t.Errorf("%s: upgrade failed: %v", c.id, resp.Diagnostics)
			continue
		}
		wantRaw, err := r.stateRaw(context.Background(), record)
		if err != nil {
			t.Fatalf("%s: %v", c.id, err)
		}
		wantGo, _ := tfval.ToGo(wantRaw)
		if !bmap.JSONEquals(got, wantGo) {
			t.Errorf("%s: upgraded state\n%s\nwant\n%s", c.id, bmap.Encode(got), bmap.Encode(wantGo))
		}
		upgraded++
	}
	if upgraded < 10 {
		t.Fatalf("only %d kinds upgraded; the state cases were not found", upgraded)
	}
}

// Reading raw JSON, the upgrader checks types itself: a known attribute with
// the wrong JSON type fails with our message, never a panic or a silent zero.
func TestAWrongTypeFailsWithTheUpgradeMessage(t *testing.T) {
	for _, tc := range []struct {
		kind string
		v0   *bmap.Map
	}{
		{"secret", bmap.MapOf("id", "s1", "name", "Key", "handle", "key", "value", "x", "maxversions", "five")},
		{"memcached", bmap.MapOf("id", "m1", "name", "Cache", "handle", "cache", "memorymb", true)},
		{"network_policy", bmap.MapOf("id", "n1", "name", "Web", "handle", "web", "ingresspeers", "frontend")},
	} {
		r := engineResource(t, tc.kind)
		resp, got := upgrade(t, r, tc.v0)
		if !resp.Diagnostics.HasError() || got != nil {
			t.Errorf("%s: a wrong type must fail the upgrade, got %s", tc.kind, bmap.Encode(got))
			continue
		}
		detail := resp.Diagnostics.Errors()[0].Detail()
		id, _ := tc.v0.Get("id")
		if !strings.Contains(detail, "bahriya_"+r.kind.TypeName()) || !strings.Contains(detail, id.(string)) {
			t.Errorf("%s: the message must name the resource and id: %s", tc.kind, detail)
		}
	}
}

// path_rule's container_id becomes container; without the rename the upgrade
// fails (negative control b).
func TestPathRuleContainerIDIsRenamed(t *testing.T) {
	r := engineResource(t, "pathrule")
	v0 := bmap.MapOf("id", "p1", "container_id", "c1", "handle", "admin", "path", "/admin")
	_, got := upgrade(t, r, v0)
	if got == nil {
		t.Fatal("upgrade failed")
	}
	if c, _ := got.Get("container"); c != "c1" {
		t.Errorf("container = %v, want c1", c)
	}

	saved := v0Renames["pathrule"]
	delete(v0Renames, "pathrule")
	defer func() { v0Renames["pathrule"] = saved }()
	if resp, _ := upgrade(t, r, v0); !resp.Diagnostics.HasError() {
		t.Error("without the rename, container_id must fail the upgrade")
	}
}

// An attribute v1 does not know fails the upgrade, naming the resource and id
// (negative control a). A deliberate drop is removed instead.
func TestUnknownAttributesFailAndDropsAreRemoved(t *testing.T) {
	r := engineResource(t, "secret")
	resp, _ := upgrade(t, r, bmap.MapOf("id", "s1", "name", "Key", "handle", "key", "value", "x", "bogus", true))
	if !resp.Diagnostics.HasError() {
		t.Fatal("an unknown attribute must fail the upgrade")
	}
	detail := resp.Diagnostics.Errors()[0].Detail()
	for _, want := range []string{"bahriya_secret", "s1", "bogus", "0.3.0 or later upgrades directly", "0.4.2"} {
		if !strings.Contains(detail, want) {
			t.Errorf("the message must name %q: %s", want, detail)
		}
	}

	c := engineResource(t, "container")
	_, got := upgrade(t, c, bmap.MapOf("id", "c1", "name", "Shop", "handle", "shop", "healthcheckscheme", "HTTP"))
	if got == nil {
		t.Fatal("a 0.3.x container state with healthcheckscheme must upgrade")
	}
	if strings.Contains(bmap.Encode(got), "healthcheckscheme") {
		t.Error("healthcheckscheme must be dropped")
	}
}

// Secrets in state (never returned by the API) carry over unchanged.
func TestSecretsCarryOver(t *testing.T) {
	r := engineResource(t, "secret")
	_, got := upgrade(t, r, bmap.MapOf("id", "s1", "name", "Key", "handle", "key", "value", "s3cret"))
	if v, _ := got.Get("value"); v != "s3cret" {
		t.Errorf("value = %v, want s3cret", v)
	}
}

type stateCase struct {
	id, kind string
	input    *bmap.Map
}

// stateCases are the toYamlState conformance inputs (API records) for every
// engine kind.
func stateCases(t *testing.T) []stateCase {
	t.Helper()
	dir := os.Getenv("BAHRIYA_OPENAPI_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "packages", "bahriya-openapi")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "conformance", "*", "cases.json"))
	if len(files) == 0 {
		t.Fatalf("no conformance cases under %s", dir)
	}
	var out []stateCase
	for _, f := range files {
		data, _ := os.ReadFile(f)
		decoded, err := bmap.Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range decoded.([]any) {
			c := raw.(*bmap.Map)
			typ, _ := c.Get("type")
			kind, _ := c.Get("kind")
			if typ != "toYamlState" {
				continue
			}
			if _, served := Descriptors[kind.(string)]; !served {
				continue
			}
			id, _ := c.Get("id")
			in, _ := c.Get("input")
			out = append(out, stateCase{id: id.(string), kind: kind.(string), input: in.(*bmap.Map)})
		}
	}
	return out
}
