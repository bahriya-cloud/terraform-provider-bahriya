package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// planToValkeyPayload must OMIT optional booleans the practitioner never set,
// so the API contract defaults govern. authenabled defaults to TRUE
// server-side — a forced false here silently created unauthenticated
// instances (found live in the 2026-08 production e2e).

func TestPlanToValkeyPayloadOmitsUnsetBooleans(t *testing.T) {
	m := &valkeyModel{
		Handle:   types.StringValue("cache-a"),
		Name:     types.StringValue("Cache A"),
		Tier:     types.StringValue("single"),
		Purpose:  types.StringValue("cache"),
		Memorymb: types.Int64Value(256),
	}

	payload, diags := planToValkeyPayload(context.Background(), m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	for _, key := range []string{"authenabled", "backupenabled", "externalenabled", "tlsenabled"} {
		if _, present := payload[key]; present {
			t.Errorf("%s must be omitted when unset so the API default applies, got %v", key, payload[key])
		}
	}
}

func TestPlanToValkeyPayloadSendsExplicitBooleans(t *testing.T) {
	m := &valkeyModel{
		Handle:          types.StringValue("cache-b"),
		Name:            types.StringValue("Cache B"),
		Tier:            types.StringValue("single"),
		Purpose:         types.StringValue("cache"),
		Memorymb:        types.Int64Value(256),
		Authenabled:     types.BoolValue(false),
		Backupenabled:   types.BoolValue(false),
		Externalenabled: types.BoolValue(true),
		Tlsenabled:      types.BoolValue(true),
	}

	payload, diags := planToValkeyPayload(context.Background(), m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	expected := map[string]bool{
		"authenabled":     false,
		"backupenabled":   false,
		"externalenabled": true,
		"tlsenabled":      true,
	}
	for key, want := range expected {
		got, present := payload[key]
		if !present {
			t.Errorf("%s must be sent when explicitly set", key)
			continue
		}
		if got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
}

func TestKnownValueFallsBackWhenPlanUnknown(t *testing.T) {
	// Sensitive attributes are Optional+Computed with no plan modifier, so the
	// framework plans them UNKNOWN whenever they are absent from config — and an
	// unknown value must never reach final state. The API never returns them, so
	// there is nothing to read back either; the prior value is the only answer.
	//
	// Generated code applies this to EVERY sensitive attribute (valkey password,
	// tls_bundle key, gpg/ssh private keys, ...), not just the password it was
	// first written for.
	prior := types.StringValue("kept-from-state")
	if got := knownValue(types.StringUnknown(), prior); !got.Equal(prior) {
		t.Errorf("unknown plan must fall back to prior, got %v", got)
	}
	if got := knownValue(types.StringUnknown(), types.StringNull()); !got.IsNull() {
		t.Errorf("unknown plan with null prior must be null, got %v", got)
	}
	explicit := types.StringValue("user-set")
	if got := knownValue(explicit, prior); !got.Equal(explicit) {
		t.Errorf("explicit plan value must win, got %v", got)
	}
	if got := knownValue(types.StringNull(), prior); !got.IsNull() {
		t.Errorf("explicitly null plan must stay null, got %v", got)
	}
}
