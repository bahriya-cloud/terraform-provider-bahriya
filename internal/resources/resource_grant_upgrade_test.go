package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// v0.x state (touser, resourcetype, resourceid) upgrades to the v1 names with
// every value carried over, so no grant is replaced on the upgrade.
func TestResourceGrantStateUpgradesFromV0(t *testing.T) {
	r := &resourceGrantResource{}
	up := r.UpgradeState(context.Background())[0]

	v0Type := up.PriorSchema.Type().TerraformType(context.Background())
	strs := tftypes.Set{ElementType: tftypes.String}
	list := tftypes.List{ElementType: tftypes.String}
	raw := tftypes.NewValue(v0Type, map[string]tftypes.Value{
		"id":           tftypes.NewValue(tftypes.String, "attachables_registries|r1|u1"),
		"touser":       tftypes.NewValue(tftypes.String, "u1"),
		"resourcetype": tftypes.NewValue(tftypes.String, "attachables_registries"),
		"resourceid":   tftypes.NewValue(tftypes.String, "r1"),
		"permissions":  tftypes.NewValue(strs, []tftypes.Value{tftypes.NewValue(tftypes.String, "read")}),
		"grant_ids":    tftypes.NewValue(list, []tftypes.Value{tftypes.NewValue(tftypes.String, "g1")}),
	})

	var sresp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &sresp)
	resp := resource.UpgradeStateResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: tftypes.NewValue(sresp.Schema.Type().TerraformType(context.Background()), nil)}}
	up.StateUpgrader(context.Background(), resource.UpgradeStateRequest{State: &tfsdk.State{Schema: *up.PriorSchema, Raw: raw}}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	var got resourceGrantModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatal(diags)
	}
	if got.Touser.ValueString() != "u1" || got.Resourcetype.ValueString() != "attachables_registries" || got.Resourceid.ValueString() != "r1" || got.ID.ValueString() != "attachables_registries|r1|u1" {
		t.Errorf("upgraded state lost a value: %+v", got)
	}
	if sresp.Schema.Version != 1 {
		t.Errorf("schema version %d, want 1", sresp.Schema.Version)
	}
}
