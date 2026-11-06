package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// newHandleRequired refuses, at plan time, a change to a create-only field
// (x-createOnly) while the handle stays the same. Such a change plans a
// replacement: Terraform terminates the item, then creates a new one with the
// same handle. A terminated item keeps its handle, so that create fails with
// "Handle already in use" and the configuration is left with nothing — for a
// database, its data gone. Changing the handle in the same plan makes the
// replacement work, so that stays allowed.
func newHandleRequired(kind, noun string) planmodifier.String {
	return newHandleRequiredModifier{kind: kind, noun: noun}
}

// newHandleRequiredBool is newHandleRequired for boolean fields.
func newHandleRequiredBool(kind, noun string) planmodifier.Bool {
	return newHandleRequiredModifier{kind: kind, noun: noun}
}

type newHandleRequiredModifier struct {
	kind, noun string
}

func (m newHandleRequiredModifier) Description(context.Context) string {
	return "Changing this value replaces the item, which needs a new handle."
}

func (m newHandleRequiredModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m newHandleRequiredModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.PlanValue.IsUnknown() || req.PlanValue.Equal(req.StateValue) {
		return
	}
	m.check(ctx, req.Path, req.Plan, req.State, &resp.Diagnostics)
}

func (m newHandleRequiredModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.StateValue.IsNull() || req.PlanValue.IsUnknown() || req.PlanValue.Equal(req.StateValue) {
		return
	}
	m.check(ctx, req.Path, req.Plan, req.State, &resp.Diagnostics)
}

// check refuses the change when the planned handle equals the current one.
func (m newHandleRequiredModifier) check(ctx context.Context, at path.Path, plan tfsdk.Plan, state tfsdk.State, diags *diag.Diagnostics) {
	var planned, current types.String
	diags.Append(plan.GetAttribute(ctx, path.Root("handle"), &planned)...)
	diags.Append(state.GetAttribute(ctx, path.Root("handle"), &current)...)
	if diags.HasError() || planned.IsUnknown() || !planned.Equal(current) {
		return
	}
	diags.AddAttributeError(at, "This change needs a new handle",
		fmt.Sprintf("%s is fixed when a %s is created, so changing it replaces the %s. Terraform would terminate this %s "+
			"and then fail to create the new one, because a terminated %s keeps its handle (%q): the %s and its data "+
			"would be gone. To replace it, give it a new handle in the same change; otherwise keep %s as it is.",
			at, m.kind, m.noun, m.noun, m.noun, current.ValueString(), m.noun, at))
}
