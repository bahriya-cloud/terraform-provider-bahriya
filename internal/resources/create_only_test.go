package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// mysqlValue is a bahriya_mysql object with every attribute null except handle.
func mysqlValue(t *testing.T, s schema.Schema, handle string) tftypes.Value {
	t.Helper()
	objType := s.Type().TerraformType(context.Background()).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for name, typ := range objType.AttributeTypes {
		values[name] = tftypes.NewValue(typ, nil)
	}
	values["handle"] = tftypes.NewValue(tftypes.String, handle)
	return tftypes.NewValue(objType, values)
}

// A create-only change that keeps the handle is refused: the replacement would
// terminate the instance and then fail on the handle a terminated instance
// keeps. With a new handle it is a working replacement and is allowed.
func TestCreateOnlyChangeNeedsANewHandle(t *testing.T) {
	var sresp resource.SchemaResponse
	(&mysqlResource{}).Schema(context.Background(), resource.SchemaRequest{}, &sresp)
	s := sresp.Schema

	tier, ok := s.Attributes["tier"].(schema.StringAttribute)
	if !ok || len(tier.PlanModifiers) != 2 {
		t.Fatalf("tier must carry RequiresReplace and newHandleRequired, got %#v", s.Attributes["tier"])
	}
	enc, ok := s.Attributes["encryptionenabled"].(schema.BoolAttribute)
	if !ok || len(enc.PlanModifiers) != 2 {
		t.Fatalf("encryptionenabled must carry RequiresReplace and newHandleRequiredBool, got %#v", s.Attributes["encryptionenabled"])
	}

	run := func(planHandle string, was, now string) bool {
		req := planmodifier.StringRequest{
			Path:       path.Root("tier"),
			StateValue: types.StringValue(was),
			PlanValue:  types.StringValue(now),
			Plan:       tfsdk.Plan{Schema: s, Raw: mysqlValue(t, s, planHandle)},
			State:      tfsdk.State{Schema: s, Raw: mysqlValue(t, s, "orders")},
		}
		var resp planmodifier.StringResponse
		newHandleRequired("mysql", "instance").PlanModifyString(context.Background(), req, &resp)
		return resp.Diagnostics.HasError()
	}
	if !run("orders", "single", "replicated") {
		t.Error("a tier change keeping the handle must be refused")
	}
	if run("orders-two", "single", "replicated") {
		t.Error("a tier change with a new handle must be allowed")
	}
	if run("orders", "single", "single") {
		t.Error("an unchanged tier must not be refused")
	}

	breq := planmodifier.BoolRequest{
		Path:       path.Root("encryptionenabled"),
		StateValue: types.BoolValue(false),
		PlanValue:  types.BoolValue(true),
		Plan:       tfsdk.Plan{Schema: s, Raw: mysqlValue(t, s, "orders")},
		State:      tfsdk.State{Schema: s, Raw: mysqlValue(t, s, "orders")},
	}
	var bresp planmodifier.BoolResponse
	newHandleRequiredBool("mysql", "instance").PlanModifyBool(context.Background(), breq, &bresp)
	if !bresp.Diagnostics.HasError() {
		t.Error("an encryption change keeping the handle must be refused")
	}
}
