package engine

import (
	"context"
	"fmt"
	"sort"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/bmap"
	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/tfval"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// SchemaVersion is the v1 engine schema version. Every v0.x resource is
// version 0: its state is API-named and flat.
const SchemaVersion = 1

// v0Renames are the v0 attribute names that are not the API's, by kind: the
// only one is path_rule's container_id (TF v1 card 10). Every other v0.4.2
// attribute of an engine kind is its API field name, checked against the
// frozen v0.4.2 schema by the upgrade tests.
var v0Renames = map[string]map[string]string{
	"pathrule": {"container_id": "container"},
}

// v0Drops are v0 attributes that v1 drops on purpose, by kind, each with why.
// A state carrying any other attribute v1 does not know fails the upgrade.
var v0Drops = map[string]map[string]string{
	// Present up to v0.3.8, gone from the API and the provider since v0.4.0:
	// the platform no longer offers a health check scheme.
	"container": {"healthcheckscheme": "removed from the API in v0.4.0"},
}

// v0UpgradePath is what a refused upgrade tells the user. State from 0.3.0 on
// upgrades directly (its schemas are tested); only older state, or state that
// was edited by hand, can be refused.
const v0UpgradePath = "State written by provider 0.3.0 or later upgrades directly; for older state, " +
	"apply once with provider 0.4.2 and run `terraform apply -refresh-only`, then switch to v1."

// UpgradeState moves v0.x state (schema version 0) to the v1 shape. It reads
// the raw state itself rather than through a prior schema, so a state from
// any v0 release is either mapped by the tables above or refused with a
// message naming the resource, never rejected by a generic decode error and
// never silently dropped.
func (r *Resource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {StateUpgrader: r.upgradeFromV0},
	}
}

func (r *Resource) upgradeFromV0(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	if req.RawState == nil || len(req.RawState.JSON) == 0 {
		resp.Diagnostics.AddError("State upgrade failed", fmt.Sprintf("bahriya_%s: the v0 state has no JSON to upgrade.", r.kind.TypeName()))
		return
	}
	record, err := r.v0Record(req.RawState.JSON)
	if err != nil {
		resp.Diagnostics.AddError("State upgrade failed", err.Error())
		return
	}
	raw, err := tfval.FromGo(r.toState(record), r.objectType(ctx))
	if err != nil {
		resp.Diagnostics.AddError("State upgrade failed",
			fmt.Sprintf("bahriya_%s %v: a value in the state written by provider v0 does not fit v1 (%v). "+v0UpgradePath, r.kind.TypeName(), mustGet(record, "id"), err))
		return
	}
	resp.State.Raw = raw
}

// v0Record turns v0 state JSON into an API-named record: renames applied,
// deliberate drops removed, anything else unknown refused.
func (r *Resource) v0Record(stateJSON []byte) (*bmap.Map, error) {
	decoded, err := bmap.Decode(stateJSON)
	if err != nil {
		return nil, fmt.Errorf("bahriya_%s: the v0 state is not valid JSON: %v", r.kind.TypeName(), err)
	}
	v0, ok := decoded.(*bmap.Map)
	if !ok {
		return nil, fmt.Errorf("bahriya_%s: the v0 state is not an object", r.kind.TypeName())
	}
	record := bmap.NewMap()
	var unknown []string
	for _, key := range v0.Keys() {
		value, _ := v0.Get(key)
		if _, dropped := v0Drops[r.desc.Name][key]; dropped {
			continue
		}
		name := key
		if to, renamed := v0Renames[r.desc.Name][key]; renamed {
			name = to
		}
		if r.kind.Field(name) == nil {
			unknown = append(unknown, key)
			continue
		}
		record.Set(name, value)
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		id, _ := v0.Get("id")
		return nil, fmt.Errorf("bahriya_%s %v: state written by provider v0 has attribute(s) %v, which v1 cannot map. "+v0UpgradePath, r.kind.TypeName(), id, unknown)
	}
	return record, nil
}
