package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// v0.x state (issystem) upgrades to the v1 name (is_system) with every value
// carried over, so no role is replaced on the upgrade (ADR 2026-11-08).
func TestRoleStateUpgradesFromV0(t *testing.T) {
	r := &roleResource{}
	up := r.UpgradeState(context.Background())[0]

	v0Type := up.PriorSchema.Type().TerraformType(context.Background())
	permType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"level": tftypes.String, "resource": tftypes.String, "permission": tftypes.String}}
	raw := tftypes.NewValue(v0Type, map[string]tftypes.Value{
		"id":          tftypes.NewValue(tftypes.String, "r1"),
		"handle":      tftypes.NewValue(tftypes.String, "deployer"),
		"name":        tftypes.NewValue(tftypes.String, "Deployer"),
		"description": tftypes.NewValue(tftypes.String, "Deploys"),
		"issystem":    tftypes.NewValue(tftypes.Bool, true),
		"permissions": tftypes.NewValue(tftypes.List{ElementType: permType}, []tftypes.Value{
			tftypes.NewValue(permType, map[string]tftypes.Value{
				"level":      tftypes.NewValue(tftypes.String, "project"),
				"resource":   tftypes.NewValue(tftypes.String, "deployables_container_http"),
				"permission": tftypes.NewValue(tftypes.String, "update"),
			}),
		}),
		"created": tftypes.NewValue(tftypes.String, "2026-11-04T10:00:00Z"),
		"updated": tftypes.NewValue(tftypes.String, nil),
	})

	var sresp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &sresp)
	resp := resource.UpgradeStateResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: tftypes.NewValue(sresp.Schema.Type().TerraformType(context.Background()), nil)}}
	up.StateUpgrader(context.Background(), resource.UpgradeStateRequest{State: &tfsdk.State{Schema: *up.PriorSchema, Raw: raw}}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	var got roleModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatal(diags)
	}
	if !got.Issystem.ValueBool() || got.ID.ValueString() != "r1" || got.Name.ValueString() != "Deployer" || len(got.Permissions.Elements()) != 1 {
		t.Errorf("upgraded state lost a value: %+v", got)
	}
	if sresp.Schema.Version != 1 {
		t.Errorf("schema version %d, want 1", sresp.Schema.Version)
	}
	if _, ok := sresp.Schema.Attributes["is_system"]; !ok {
		t.Error("v1 names it is_system")
	}
}
