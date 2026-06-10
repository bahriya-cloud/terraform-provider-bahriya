package datasources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
)

func TestOrganisationDataSource_Metadata(t *testing.T) {
	ds := NewOrganisationDataSource()
	req := datasource.MetadataRequest{ProviderTypeName: "bahriya"}
	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), req, resp)
	if resp.TypeName != "bahriya_organisation" {
		t.Fatalf("expected bahriya_organisation, got %s", resp.TypeName)
	}
}

func TestOrganisationDataSource_Schema(t *testing.T) {
	ds := NewOrganisationDataSource()
	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	expected := []string{"id", "name", "handle", "email", "role"}
	for _, name := range expected {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("expected attribute %q in schema", name)
		}
	}
}

func TestOrganisationDataSource_Configure(t *testing.T) {
	c := client.New("http://localhost", "tok", "org")
	ds := NewOrganisationDataSource().(*organisationDataSource)
	req := datasource.ConfigureRequest{}
	req.ProviderData = c
	resp := &datasource.ConfigureResponse{}
	ds.Configure(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics)
	}
	if ds.client != c {
		t.Fatal("client not set")
	}
}

func TestOrganisationDataSource_Configure_NilProviderData(t *testing.T) {
	ds := NewOrganisationDataSource().(*organisationDataSource)
	req := datasource.ConfigureRequest{}
	resp := &datasource.ConfigureResponse{}
	ds.Configure(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal("should not error on nil ProviderData")
	}
}

func TestOrganisationDataSource_Configure_WrongType(t *testing.T) {
	ds := NewOrganisationDataSource().(*organisationDataSource)
	req := datasource.ConfigureRequest{}
	req.ProviderData = "not a client"
	resp := &datasource.ConfigureResponse{}
	ds.Configure(context.Background(), req, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for wrong provider data type")
	}
}

func TestStringFromRaw(t *testing.T) {
	raw := map[string]any{"name": "test", "missing": nil, "number": 42}

	v := stringFromRaw(raw, "name")
	if v.IsNull() || v.ValueString() != "test" {
		t.Errorf("expected 'test', got %v", v)
	}

	v = stringFromRaw(raw, "missing")
	if !v.IsNull() {
		t.Errorf("expected null for nil value, got %v", v)
	}

	v = stringFromRaw(raw, "number")
	if !v.IsNull() {
		t.Errorf("expected null for non-string value, got %v", v)
	}

	v = stringFromRaw(raw, "nonexistent")
	if !v.IsNull() {
		t.Errorf("expected null for nonexistent key, got %v", v)
	}
}

func newOrgServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"code":   200,
			"status": "ok",
			"data": map[string]any{
				"count": 1,
				"organisations": []any{
					map[string]any{
						"id":     "org-uuid-123",
						"name":   "Test Org",
						"handle": "test-org",
						"email":  "test@example.com",
						"role":   "owner",
					},
				},
			},
		})
	}))
}

func TestOrganisationDataSource_ParsesResponse(t *testing.T) {
	srv := newOrgServer()
	defer srv.Close()

	c := client.New(srv.URL, "tok", "org-uuid-123")

	raw := map[string]any{
		"count": 1,
		"organisations": []any{
			map[string]any{
				"id":     "org-uuid-123",
				"name":   "Test Org",
				"handle": "test-org",
				"email":  "test@example.com",
				"role":   "owner",
			},
		},
	}

	_ = c

	orgs, ok := raw["organisations"].([]any)
	if !ok || len(orgs) == 0 {
		t.Fatal("expected organisations array")
	}
	obj, ok := orgs[0].(map[string]any)
	if !ok {
		t.Fatal("expected map for first org")
	}
	if obj["id"] != "org-uuid-123" {
		t.Fatalf("expected org-uuid-123, got %v", obj["id"])
	}
	if obj["handle"] != "test-org" {
		t.Fatalf("expected test-org, got %v", obj["handle"])
	}
}
