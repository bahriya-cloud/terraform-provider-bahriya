package resources

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func planRotateOnly(state, plan types.String) planmodifier.StringResponse {
	req := planmodifier.StringRequest{Path: path.Root("cert"), StateValue: state, PlanValue: plan, ConfigValue: plan}
	resp := planmodifier.StringResponse{PlanValue: plan}
	rotateOnly("tls_bundle").PlanModifyString(context.Background(), req, &resp)
	return resp
}

// #2620: a changed certificate is refused in the plan, naming the way to rotate it.
func TestRotateOnlyRefusesAChange(t *testing.T) {
	resp := planRotateOnly(types.StringValue("cert-a"), types.StringValue("cert-b"))
	if !resp.Diagnostics.HasError() {
		t.Fatal("a material change must be refused at plan time")
	}
	detail := resp.Diagnostics.Errors()[0].Detail()
	if !strings.Contains(detail, "reis tls_bundle:rotate") || !strings.Contains(detail, "provider v1") {
		t.Errorf("the error must say how to rotate and that v1 will: %q", detail)
	}
}

// encryption_key format is fixed at creation: refused, never replaced.
func TestFixedAtCreationRefusesAChangeAndAllowsCreate(t *testing.T) {
	run := func(state, plan types.String) planmodifier.StringResponse {
		req := planmodifier.StringRequest{Path: path.Root("format"), StateValue: state, PlanValue: plan, ConfigValue: plan}
		resp := planmodifier.StringResponse{PlanValue: plan}
		fixedAtCreation("encryption_key").PlanModifyString(context.Background(), req, &resp)
		return resp
	}

	resp := run(types.StringValue("base64"), types.StringValue("hex"))
	if !resp.Diagnostics.HasError() {
		t.Fatal("a format change must be refused at plan time")
	}
	if detail := resp.Diagnostics.Errors()[0].Detail(); detail != "format is fixed when a key is created; to use another format, create a new key and move its attachments." {
		t.Errorf("unexpected detail %q", detail)
	}
	if resp := run(types.StringNull(), types.StringValue("hex")); resp.Diagnostics.HasError() {
		t.Error("create must be allowed")
	}
	if resp := run(types.StringValue("hex"), types.StringValue("hex")); resp.Diagnostics.HasError() {
		t.Error("an unchanged value must be allowed")
	}
}

func TestRotateOnlyAllowsCreateUnchangedAndUnknown(t *testing.T) {
	cases := map[string][2]types.String{
		"create (no prior value)":           {types.StringNull(), types.StringValue("cert-a")},
		"unchanged":                         {types.StringValue("cert-a"), types.StringValue("cert-a")},
		"unknown until apply":               {types.StringValue("cert-a"), types.StringUnknown()},
		"imported, write-only not in state": {types.StringNull(), types.StringValue("key-a")},
	}
	for name, c := range cases {
		if resp := planRotateOnly(c[0], c[1]); resp.Diagnostics.HasError() {
			t.Errorf("%s: unexpected error %v", name, resp.Diagnostics)
		}
	}
}
