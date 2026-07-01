package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// apiToPathRuleModel + planToPathRulePayload form a round-trip pair on
// the wire. These tests cover the smushed field shape and the credential
// list handling, which is the trickiest part of the resource.

func TestApiToPathRuleModelBasicFields(t *testing.T) {
	raw := map[string]any{
		"id":       "11111111-1111-1111-1111-111111111111",
		"handle":   "admin",
		"path":     "/api/admin",
		"priority": float64(100),

		"ratelimitingenabled":           true,
		"ratelimitingrequestspersecond": float64(5),
		"ratelimitingrequestsperminute": float64(60),
		"ratelimitingrequestsperhour":   float64(1000),

		"ipwhitelistenabled": false,
		"ipwhitelist":        []any{"10.0.0.0/8", "1.2.3.4"},

		"ipblacklistenabled": true,
		"ipblacklist":        []any{"5.6.7.8"},

		"basicauthenabled": true,
		"basicauthcredentials": []any{
			map[string]any{"username": "alice", "password": "value-hidden-for-your-own-good"},
		},
	}

	m := apiToPathRuleModel(raw, "container-abc")

	if got := m.ID.ValueString(); got != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", got)
	}
	if got := m.ContainerID.ValueString(); got != "container-abc" {
		t.Errorf("ContainerID = %q", got)
	}
	if got := m.Handle.ValueString(); got != "admin" {
		t.Errorf("Handle = %q", got)
	}
	if got := m.Path.ValueString(); got != "/api/admin" {
		t.Errorf("Path = %q", got)
	}
	if got := m.Priority.ValueInt64(); got != 100 {
		t.Errorf("Priority = %d", got)
	}
	if !m.Ratelimitingenabled.ValueBool() {
		t.Error("Ratelimitingenabled expected true")
	}
	if got := m.Ratelimitingrequestspersecond.ValueInt64(); got != 5 {
		t.Errorf("RPS = %d", got)
	}
	if got := m.Ratelimitingrequestsperminute.ValueInt64(); got != 60 {
		t.Errorf("RPM = %d", got)
	}
	if got := m.Ratelimitingrequestsperhour.ValueInt64(); got != 1000 {
		t.Errorf("RPH = %d", got)
	}
	if m.Ipwhitelistenabled.ValueBool() {
		t.Error("Ipwhitelistenabled expected false")
	}
	if got := len(m.Ipwhitelist.Elements()); got != 2 {
		t.Errorf("ipwhitelist len = %d", got)
	}
	if !m.Ipblacklistenabled.ValueBool() {
		t.Error("Ipblacklistenabled expected true")
	}
	if !m.Basicauthenabled.ValueBool() {
		t.Error("Basicauthenabled expected true")
	}
	if got := len(m.Basicauthcredentials.Elements()); got != 1 {
		t.Errorf("creds len = %d", got)
	}
}

func TestApiToPathRuleModelNullsForMissingKeys(t *testing.T) {
	// Empty payload → all nullable fields land as null, no panics.
	m := apiToPathRuleModel(map[string]any{}, "container-abc")

	if !m.ID.IsNull() {
		t.Error("ID expected null")
	}
	if !m.Handle.IsNull() {
		t.Error("Handle expected null")
	}
	if !m.Priority.IsNull() {
		t.Error("Priority expected null")
	}
	if !m.Ratelimitingrequestsperminute.IsNull() {
		t.Error("RPM expected null when absent")
	}
}

func TestPlanToPathRulePayloadFlatShape(t *testing.T) {
	allowList, _ := types.ListValue(types.StringType, []attr.Value{
		types.StringValue("10.0.0.0/8"),
	})
	creds, _ := types.ListValue(pathRuleCredentialObjectType(), []attr.Value{
		mustObject(t, "alice", "s3cret"),
	})

	m := pathRuleModel{
		Handle:                        types.StringValue("admin"),
		Path:                          types.StringValue("/api/admin"),
		Priority:                      types.Int64Value(10),
		Ratelimitingenabled:           types.BoolValue(true),
		Ratelimitingrequestsperminute: types.Int64Value(60),
		Ratelimitingrequestspersecond: types.Int64Null(),
		Ratelimitingrequestsperhour:   types.Int64Null(),
		Ipwhitelistenabled:            types.BoolValue(true),
		Ipwhitelist:                   allowList,
		Ipblacklistenabled:            types.BoolValue(false),
		Ipblacklist:                   types.ListNull(types.StringType),
		Basicauthenabled:              types.BoolValue(true),
		Basicauthcredentials:          creds,
	}

	out, _ := planToPathRulePayload(context.Background(), &m)

	if out["handle"] != "admin" {
		t.Errorf("handle = %v", out["handle"])
	}
	if out["path"] != "/api/admin" {
		t.Errorf("path = %v", out["path"])
	}
	if out["priority"] != 10 {
		t.Errorf("priority = %v", out["priority"])
	}
	if out["ratelimitingrequestsperminute"] != int64(60) {
		t.Errorf("RPM = %v", out["ratelimitingrequestsperminute"])
	}
	if out["ratelimitingrequestspersecond"] != nil {
		t.Errorf("RPS expected nil, got %v", out["ratelimitingrequestspersecond"])
	}
	allow, ok := out["ipwhitelist"].([]string)
	if !ok || len(allow) != 1 || allow[0] != "10.0.0.0/8" {
		t.Errorf("ipwhitelist = %v", out["ipwhitelist"])
	}
	if out["ipblacklist"] != nil {
		t.Errorf("ipblacklist expected nil, got %v", out["ipblacklist"])
	}
	credList, ok := out["basicauthcredentials"].([]map[string]any)
	if !ok || len(credList) != 1 {
		t.Fatalf("creds = %v", out["basicauthcredentials"])
	}
	if credList[0]["username"] != "alice" || credList[0]["password"] != "s3cret" {
		t.Errorf("cred = %v", credList[0])
	}
}

func TestMergeCredentialsPrefersPlanWhenAPIReturnsSentinel(t *testing.T) {
	// API returns the masked sentinel for password; plan has the real one.
	// merge should keep the plan's password so subsequent applies don't drift.
	plan, _ := types.ListValue(pathRuleCredentialObjectType(), []attr.Value{
		mustObject(t, "alice", "s3cret"),
	})
	api, _ := types.ListValue(pathRuleCredentialObjectType(), []attr.Value{
		mustObject(t, "alice", passwordSentinel),
	})

	merged := mergeCredentials(plan, api)
	if got := len(merged.Elements()); got != 1 {
		t.Fatalf("len = %d", got)
	}
	obj := merged.Elements()[0].(types.Object)
	pwd := obj.Attributes()["password"].(types.String).ValueString()
	if pwd != "s3cret" {
		t.Errorf("password = %q, want s3cret (plan-side, not sentinel)", pwd)
	}
}

func TestMergeCredentialsKeepsAPIWhenPlanNull(t *testing.T) {
	// Import path — plan is null, so we adopt the API as-is, sentinel and
	// all. The user can then `apply` with a real password to rotate.
	api, _ := types.ListValue(pathRuleCredentialObjectType(), []attr.Value{
		mustObject(t, "alice", passwordSentinel),
	})
	merged := mergeCredentials(types.ListNull(pathRuleCredentialObjectType()), api)
	if got := len(merged.Elements()); got != 1 {
		t.Fatalf("len = %d", got)
	}
}

func TestMergeCredentialsPassesActualPasswordThrough(t *testing.T) {
	// If the API somehow returns a real password (shouldn't happen but cover
	// it), we take the API value rather than the plan — the API is the source
	// of truth for what was persisted.
	plan, _ := types.ListValue(pathRuleCredentialObjectType(), []attr.Value{
		mustObject(t, "alice", "old-plan-value"),
	})
	api, _ := types.ListValue(pathRuleCredentialObjectType(), []attr.Value{
		mustObject(t, "alice", "real-from-api"),
	})
	merged := mergeCredentials(plan, api)
	obj := merged.Elements()[0].(types.Object)
	pwd := obj.Attributes()["password"].(types.String).ValueString()
	if pwd != "real-from-api" {
		t.Errorf("password = %q, want real-from-api", pwd)
	}
}

func mustObject(t *testing.T, username, password string) attr.Value {
	t.Helper()
	v, diags := types.ObjectValue(pathRuleCredentialAttrTypes(), map[string]attr.Value{
		"username": types.StringValue(username),
		"password": types.StringValue(password),
	})
	if diags.HasError() {
		t.Fatalf("build credential object: %v", diags)
	}
	return v
}
