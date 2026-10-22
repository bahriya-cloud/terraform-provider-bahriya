package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// bahriya_mysql (SR-2026-008). Databases are nested ATTRIBUTES — HCL writes
// them `databases = [{ name = "...", username = "..." }]`, never as blocks.
// Passwords are platform-generated and never part of the resource or state.

func mysqlDatabasesList(t *testing.T, pairs ...[2]string) types.List {
	t.Helper()
	var items []attr.Value
	for _, p := range pairs {
		obj, diags := types.ObjectValue(mysqlDatabasesAttrTypes(), map[string]attr.Value{
			"name":     types.StringValue(p[0]),
			"username": types.StringValue(p[1]),
		})
		if diags.HasError() {
			t.Fatalf("building database object: %v", diags)
		}
		items = append(items, obj)
	}
	list, diags := types.ListValue(mysqlDatabasesObjectType(), items)
	if diags.HasError() {
		t.Fatalf("building databases list: %v", diags)
	}
	return list
}

func TestPlanToMysqlPayloadSendsNestedDatabasesAndConfig(t *testing.T) {
	config, _ := types.MapValue(types.StringType, map[string]attr.Value{
		"max_connections": types.StringValue("300"),
		"slow_query_log":  types.StringValue("true"),
	})
	m := &mysqlModel{
		Handle:        types.StringValue("orders"),
		Name:          types.StringValue("Orders"),
		Tier:          types.StringValue("replicated"),
		Replicas:      types.Int64Value(2),
		Cpumillicores: types.Int64Value(1000),
		Memorymb:      types.Int64Value(2048),
		Storagegb:     types.Int64Value(20),
		Databases:     mysqlDatabasesList(t, [2]string{"orders", "orders-app"}, [2]string{"reporting", "reporting-ro"}),
		Config:        config,
	}

	payload, diags := planToMysqlPayload(context.Background(), m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	dbs, ok := payload["databases"].([]map[string]any)
	if !ok {
		t.Fatalf("databases must be a list of objects, got %T", payload["databases"])
	}
	if len(dbs) != 2 || dbs[0]["name"] != "orders" || dbs[0]["username"] != "orders-app" || dbs[1]["name"] != "reporting" {
		t.Errorf("databases = %v", dbs)
	}
	for key, want := range map[string]any{"tier": "replicated", "replicas": int64(2), "cpumillicores": int64(1000), "memorymb": int64(2048), "storagegb": int64(20)} {
		if payload[key] != want {
			t.Errorf("%s = %v, want %v", key, payload[key], want)
		}
	}
	cfg, ok := payload["config"].(map[string]string)
	if !ok || cfg["max_connections"] != "300" || cfg["slow_query_log"] != "true" {
		t.Errorf("config = %#v (the API accepts the string forms and stores them typed)", payload["config"])
	}
}

func TestPlanToMysqlPayloadOmitsUnsetBooleans(t *testing.T) {
	m := &mysqlModel{
		Handle:        types.StringValue("orders"),
		Name:          types.StringValue("Orders"),
		Tier:          types.StringValue("single"),
		Cpumillicores: types.Int64Value(500),
		Memorymb:      types.Int64Value(1024),
		Storagegb:     types.Int64Value(10),
		Databases:     mysqlDatabasesList(t, [2]string{"app", "app-user"}),
	}

	payload, diags := planToMysqlPayload(context.Background(), m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	for _, key := range []string{"tlsenabled", "encryptionenabled", "externalenabled", "clientenabled", "replicas"} {
		if _, present := payload[key]; present {
			t.Errorf("%s must be omitted when unset so the API default applies, got %v", key, payload[key])
		}
	}
}

func TestApiToMysqlModelMapsTypedConfigAndDatabases(t *testing.T) {
	m := apiToMysqlModel(map[string]any{
		"id":            "my-1",
		"handle":        "orders",
		"tier":          "ha",
		"replicas":      float64(3),
		"nodes":         float64(3),
		"externalport":  float64(30117),
		"cpumillicores": float64(1000),
		"databases":     []any{map[string]any{"name": "orders", "username": "orders-app"}},
		// The API stores config typed; state carries the string forms the
		// practitioner wrote, so a typed read-back must not cause drift.
		"config": map[string]any{"max_connections": float64(300), "slow_query_log": true, "sql_mode": "STRICT_TRANS_TABLES"},
	})

	if m.Nodes.ValueInt64() != 3 || m.Externalport.ValueInt64() != 30117 {
		t.Errorf("computed nodes/externalport not read: %v / %v", m.Nodes, m.Externalport)
	}
	var cfg map[string]string
	if diags := m.Config.ElementsAs(context.Background(), &cfg, false); diags.HasError() {
		t.Fatalf("config elements: %v", diags)
	}
	want := map[string]string{"max_connections": "300", "slow_query_log": "true", "sql_mode": "STRICT_TRANS_TABLES"}
	for k, v := range want {
		if cfg[k] != v {
			t.Errorf("config[%s] = %q, want %q", k, cfg[k], v)
		}
	}
	var dbs []mysqlDatabasesModel
	if diags := m.Databases.ElementsAs(context.Background(), &dbs, false); diags.HasError() {
		t.Fatalf("databases elements: %v", diags)
	}
	if len(dbs) != 1 || dbs[0].Username.ValueString() != "orders-app" {
		t.Errorf("databases = %v", dbs)
	}
}

func TestMysqlSchemaContract(t *testing.T) {
	r := &mysqlResource{}
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	attrs := resp.Schema.Attributes

	// Tier and encryption are create-time decisions: changing either must
	// plan a replacement, never an in-place update the API would refuse.
	tier, ok := attrs["tier"].(schema.StringAttribute)
	if !ok || len(tier.PlanModifiers) == 0 {
		t.Errorf("tier must carry RequiresReplace")
	}
	enc, ok := attrs["encryptionenabled"].(schema.BoolAttribute)
	if !ok || len(enc.PlanModifiers) == 0 {
		t.Errorf("encryptionenabled must carry a bool RequiresReplace")
	}
	// The LUKS add-on (ADR 2026-10-07) is create-time too.
	vol, ok := attrs["volumeencryptionenabled"].(schema.BoolAttribute)
	if !ok || len(vol.PlanModifiers) == 0 {
		t.Errorf("volumeencryptionenabled must carry a bool RequiresReplace")
	}
	// The managed vault items are read-only: shown, never set.
	for _, managed := range []string{"rootsecret", "lukskey", "keyringkey", "keyrotationstatus"} {
		a, ok := attrs[managed].(schema.StringAttribute)
		if !ok || !a.Computed || a.Optional || a.Required {
			t.Errorf("%q must be a computed-only string attribute", managed)
		}
	}
	if a, ok := attrs["databasesecrets"].(schema.ListAttribute); !ok || !a.Computed || a.Optional {
		t.Errorf("databasesecrets must be a computed-only list attribute, got %T", attrs["databasesecrets"])
	}

	if _, ok := attrs["databases"].(schema.ListNestedAttribute); !ok {
		t.Errorf("databases must be a ListNestedAttribute (attribute syntax), got %T", attrs["databases"])
	}
	for _, computed := range []string{"externalport", "nodes", "status"} {
		if _, ok := attrs[computed]; !ok {
			t.Errorf("schema missing computed attribute %q", computed)
		}
	}
	for name := range attrs {
		if name == "password" || name == "rootpassword" {
			t.Errorf("no credential may ever be a resource attribute (%q)", name)
		}
	}
}
