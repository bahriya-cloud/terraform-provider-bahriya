package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/bmap"
	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/tfval"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// ADR 2026-11-08 (the resource shape is settled before v1 freezes it): what the
// 0 → 1 upgrade does with numbers v0 kept as strings, and with an http
// container whose v0 configuration never stated externalnetworkingenabled.

// v042Container is an http container as provider 0.4.2 wrote it: the API's
// record under API names, numbers as strings, every default the API filled on
// create, and external as the API stored it (true, because the API fills true
// for http when it is not sent).
func v042Container(t *testing.T) *bmap.Map {
	t.Helper()
	k := bmap.Embedded().Kind("container")
	record, err := k.ToAPI(bmap.MapOf(
		"name", "Shop", "handle", "shop", "regions", []any{"helsinki-1"},
		"workload", bmap.MapOf("image", "nginx:1.27", "cpu", int64(250), "memory", int64(256)),
		"scaling", bmap.MapOf("replicas", int64(2)),
		"networking", bmap.MapOf("external", true),
	), true)
	if err != nil {
		t.Fatal(err)
	}
	for _, kv := range [][2]any{{"id", "c1"}, {"type", "http"}, {"maxcpu", "313"}, {"maxmemory", "320"}} {
		record.Set(kv[0].(string), kv[1])
	}
	return record
}

func TestV0NumberStringsBecomeIntegers(t *testing.T) {
	r := engineResource(t, "container")
	resp, got := upgrade(t, r, v042Container(t))
	if got == nil {
		t.Fatalf("upgrade failed: %v", resp.Diagnostics)
	}
	workload, _ := got.Get("workload")
	scaling, _ := got.Get("scaling")
	for path, want := range map[string]int64{"workload.cpu": 250, "workload.max_cpu": 313, "workload.memory": 256, "workload.port": 8080, "scaling.replicas": 2} {
		group := workload
		if strings.HasPrefix(path, "scaling.") {
			group = scaling
		}
		v, _ := group.(*bmap.Map).Get(strings.SplitN(path, ".", 2)[1])
		if n, ok := v.(int64); !ok || n != want {
			t.Errorf("%s = %#v, want the integer %d", path, v, want)
		}
	}
}

func TestAV0NumberThatIsNotANumberFailsTheUpgrade(t *testing.T) {
	r := engineResource(t, "container")
	v0 := v042Container(t)
	v0.Set("mincpu", "a quarter")
	resp, got := upgrade(t, r, v0)
	if got != nil || !resp.Diagnostics.HasError() {
		t.Fatalf("a non-number in a v1 number must fail the upgrade, got %s", bmap.Encode(got))
	}
	detail := resp.Diagnostics.Errors()[0].Detail()
	for _, want := range []string{"bahriya_container", "c1", "0.4.2"} {
		if !strings.Contains(detail, want) {
			t.Errorf("the message must name %q: %s", want, detail)
		}
	}
}

// plan runs ModifyPlan with a configuration (a YAML-shaped document) over the
// upgraded state.
func plan(t *testing.T, r *Resource, config *bmap.Map, prior tftypes.Value) *resource.ModifyPlanResponse {
	t.Helper()
	ctx := context.Background()
	raw, err := tfval.FromGo(config, r.objectType(ctx))
	if err != nil {
		t.Fatal(err)
	}
	resp := &resource.ModifyPlanResponse{Plan: tfsdk.Plan{Schema: r.schema, Raw: raw}}
	r.ModifyPlan(ctx, resource.ModifyPlanRequest{
		Config: tfsdk.Config{Schema: r.schema, Raw: raw},
		Plan:   tfsdk.Plan{Schema: r.schema, Raw: raw},
		State:  tfsdk.State{Schema: r.schema, Raw: prior},
	}, resp)
	return resp
}

// An upgraded http container whose configuration leaves networking.external
// out fails the plan, naming the container and asking for true or false; with
// external = true the plan matches the upgraded state (no diff).
func TestAnUpgradedHTTPContainerMustStateExternal(t *testing.T) {
	r := engineResource(t, "container")
	resp, upgraded := upgrade(t, r, v042Container(t))
	if upgraded == nil {
		t.Fatalf("upgrade failed: %v", resp.Diagnostics)
	}

	// The configuration a user writes for that container: its YAML shape.
	record, _ := r.kind.FromState(upgraded)
	config, err := r.kind.ToYAML(record, false)
	if err != nil {
		t.Fatal(err)
	}
	config.Delete("kind")
	networking, _ := config.Get("networking")
	networking.(*bmap.Map).Delete("external")
	if networking.(*bmap.Map).Len() == 0 {
		config.Delete("networking")
	}

	refused := plan(t, r, config, resp.State.Raw)
	if !refused.Diagnostics.HasError() {
		t.Fatal("the plan must fail when networking.external is left out of an http container")
	}
	detail := refused.Diagnostics.Errors()[0].Detail()
	for _, want := range []string{"networking.external", `"shop"`, "true or false"} {
		if !strings.Contains(detail, want) {
			t.Errorf("the error must say %q: %s", want, detail)
		}
	}

	config.Set("networking", bmap.MapOf("external", true))
	accepted := plan(t, r, config, resp.State.Raw)
	if accepted.Diagnostics.HasError() {
		t.Fatalf("with external = true the plan must succeed: %v", accepted.Diagnostics)
	}
	planned, err := tfval.ToGo(accepted.Plan.Raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bmap.JSONEquals(planned, upgraded) {
		t.Errorf("the plan must equal the upgraded state (no diff)\nplan:  %s\nstate: %s", bmap.Encode(planned), bmap.Encode(upgraded))
	}
}

// Every field the schema forbids on worker or cron job containers fails the plan
// there. Released v0.4.2 let such a field through, the API cleared it, and the
// apply ended in "Provider produced inconsistent result after apply" (a create
// left tainted; an update repeating). Driven by the embedded schema, so a field
// newly forbidden (card 2985: security.rate_limiting.per_second) is covered as
// soon as its metadata lands. List-of-object fields need hand-built items and
// are listed in the log rather than set here.
func TestEveryForbiddenFieldFailsThePlanOnWorkersAndCronJobs(t *testing.T) {
	kind := bmap.Embedded().Kind("container")
	base := func(typ string) *bmap.Map {
		config := bmap.MapOf(
			"name", "Jobs", "handle", "jobs", "type", typ, "regions", []any{"helsinki-1"},
			"workload", bmap.MapOf("image", "busybox:1.37", "cpu", int64(100), "memory", int64(64)),
		)
		if typ == "cronjob" {
			config.Set("schedule", bmap.MapOf("schedule", "*/5 * * * *"))
		}
		return config
	}
	checked, skipped := 0, []string{}
	for _, typ := range []string{"worker", "cronjob"} {
		for _, f := range kind.Fields {
			if !f.Desc.TypeForbidden[typ] || f.Desc.ReadOnly || f.Desc.DisplayReadOnly {
				continue
			}
			var value any
			switch {
			case len(f.Desc.Enum) > 0:
				value = f.Desc.Enum[len(f.Desc.Enum)-1]
			case f.Desc.Type == "boolean":
				value = true
			case f.Desc.Type == "integer":
				value = int64(60)
			case f.Desc.Type == "string":
				value = "x"
			case f.Desc.Type == "array" && !f.HasItem:
				value = []any{"x"}
			default:
				skipped = append(skipped, typ+":"+f.Path)
				continue
			}
			config := base(typ)
			setPath(config, f.Path, value)
			r := engineResource(t, "container")
			resp := plan(t, r, config, tftypes.NewValue(r.objectType(context.Background()), nil))
			if !resp.Diagnostics.HasError() {
				t.Errorf("%s on a %s must fail the plan", f.Path, typ)
				continue
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no forbidden field was checked")
	}
	t.Logf("%d forbidden fields fail the plan; list-of-object fields not set here: %v", checked, skipped)
}

// setPath sets a dotted path in a nested config map, creating groups as needed.
func setPath(config *bmap.Map, path string, value any) {
	parts := strings.Split(path, ".")
	node := config
	for _, part := range parts[:len(parts)-1] {
		child, ok := node.Get(part)
		next, isMap := child.(*bmap.Map)
		if !ok || !isMap {
			next = bmap.NewMap()
			node.Set(part, next)
		}
		node = next
	}
	node.Set(parts[len(parts)-1], value)
}

// A worker that sets a field only http containers allow fails the plan.
func TestAForbiddenFieldFailsThePlan(t *testing.T) {
	r := engineResource(t, "container")
	config := bmap.MapOf(
		"name", "Jobs", "handle", "jobs", "type", "worker", "regions", []any{"helsinki-1"},
		"workload", bmap.MapOf("image", "busybox:1.37", "cpu", int64(100), "memory", int64(64)),
		"networking", bmap.MapOf("external", true),
	)
	resp := plan(t, r, config, tftypes.NewValue(r.objectType(context.Background()), nil))
	if !resp.Diagnostics.HasError() {
		t.Fatal("networking.external on a worker must fail the plan")
	}
	if summary := resp.Diagnostics.Errors()[0].Summary(); summary != "Not allowed on worker containers" {
		t.Errorf("summary = %q", summary)
	}
}
