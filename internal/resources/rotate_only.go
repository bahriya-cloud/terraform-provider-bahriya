package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// rotateOnly refuses, at plan time, a change to the material of a versioned
// item whose update only renames it (#2620). The API changes that material
// only through POST …/rotate, which this provider version does not call, so
// an in-place update would be accepted, ignored by the API, and then fail with
// "Provider produced inconsistent result after apply". Refusing it in the plan
// says why and what to do instead. Provider v1 rotates such changes itself.
//
// A null prior value (create, or an imported item whose write-only material
// is not in state) is not a change, so it is allowed.
func rotateOnly(kind string) planmodifier.String {
	return rotateOnlyModifier{kind: kind}
}

type rotateOnlyModifier struct {
	kind string
}

func (m rotateOnlyModifier) Description(context.Context) string {
	return "Changing this value requires rotating the item, which this provider version does not do."
}

func (m rotateOnlyModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m rotateOnlyModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.PlanValue.IsUnknown() || req.PlanValue.Equal(req.StateValue) {
		return
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Rotation is not supported by this provider version",
		fmt.Sprintf(
			"Changing %s on a %s replaces its material, which the API does only by rotation. "+
				"Rotate it with `reis %s:rotate` or in the console, then set this value to match. "+
				"Terraform provider v1 rotates it for you.",
			req.Path, m.kind, m.kind,
		),
	)
}

// fixedAtCreation refuses, at plan time, a change to a field the API fixes
// when the item is created (encryption_key `format`): neither an update nor a
// rotation can change it. Replacing the item instead would destroy its
// attachments as a side effect, so the change is refused with what to do.
func fixedAtCreation(kind string) planmodifier.String {
	return fixedAtCreationModifier{kind: kind}
}

type fixedAtCreationModifier struct {
	kind string
}

func (m fixedAtCreationModifier) Description(context.Context) string {
	return "Fixed when the item is created; it cannot be changed afterwards."
}

func (m fixedAtCreationModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m fixedAtCreationModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.PlanValue.IsUnknown() || req.PlanValue.Equal(req.StateValue) {
		return
	}
	noun := fixedAtCreationNouns[m.kind]
	if noun == "" {
		noun = m.kind
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"This value is fixed when the item is created",
		fmt.Sprintf(
			"%s is fixed when a %s is created; to use another %s, create a new %s and move its attachments.",
			req.Path, noun, req.Path, noun,
		),
	)
}

// fixedAtCreationNouns is how the message names the item, by kind.
var fixedAtCreationNouns = map[string]string{"encryption_key": "key"}
