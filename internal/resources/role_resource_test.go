package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// apiToRoleModel + permissionsToAPI form the wire round-trip for a role. The
// permissions nested block is the trickiest part, so it gets the most cover.

func TestApiToRoleModelFields(t *testing.T) {
	raw := map[string]any{
		"id":          "11111111-1111-1111-1111-111111111111",
		"handle":      "deployer",
		"name":        "Deployer",
		"description": "Can deploy containers",
		"issystem":    false,
		"permissions": []any{
			map[string]any{"level": "project", "resource": "deployables_container_http", "permission": "create"},
			map[string]any{"level": "organisation", "resource": "attachables_registries", "permission": "read"},
		},
		"created": "2026-07-05T10:00:00+00:00",
		"updated": "2026-07-05T10:00:00+00:00",
	}

	m := apiToRoleModel(raw)

	if m.Handle.ValueString() != "deployer" {
		t.Errorf("handle = %q", m.Handle.ValueString())
	}
	if m.Name.ValueString() != "Deployer" {
		t.Errorf("name = %q", m.Name.ValueString())
	}
	if m.Issystem.ValueBool() {
		t.Errorf("issystem should be false")
	}
	if len(m.Permissions.Elements()) != 2 {
		t.Fatalf("permissions len = %d", len(m.Permissions.Elements()))
	}
}

func TestPermissionsRoundTrip(t *testing.T) {
	raw := map[string]any{
		"permissions": []any{
			map[string]any{"level": "project", "resource": "deployables_memcached", "permission": "update"},
		},
	}

	list := permissionsFromAPI(raw, "permissions")
	out := permissionsToAPI(list)

	if len(out) != 1 {
		t.Fatalf("out len = %d", len(out))
	}
	if out[0]["level"] != "project" || out[0]["resource"] != "deployables_memcached" || out[0]["permission"] != "update" {
		t.Errorf("round-trip grant = %v", out[0])
	}
}

func TestPermissionsToAPIFromPlan(t *testing.T) {
	obj, _ := types.ObjectValue(permissionAttrTypes(), map[string]attr.Value{
		"level":      types.StringValue("organisation"),
		"resource":   types.StringValue("attachables_secrets"),
		"permission": types.StringValue("delete"),
	})
	list, _ := types.ListValue(permissionObjectType(), []attr.Value{obj})

	out := permissionsToAPI(list)
	if len(out) != 1 || out[0]["permission"] != "delete" || out[0]["resource"] != "attachables_secrets" {
		t.Errorf("plan → api = %v", out)
	}
}

func TestPermissionsToAPINull(t *testing.T) {
	if got := permissionsToAPI(types.ListNull(permissionObjectType())); len(got) != 0 {
		t.Errorf("null list should marshal to empty slice, got %v", got)
	}
}
