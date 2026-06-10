package datasources

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
)

func TestRegionDataSource_Metadata(t *testing.T) {
	ds := NewRegionDataSource()
	req := datasource.MetadataRequest{ProviderTypeName: "bahriya"}
	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), req, resp)
	if resp.TypeName != "bahriya_region" {
		t.Fatalf("expected bahriya_region, got %s", resp.TypeName)
	}
}

func TestRegionDataSource_Schema(t *testing.T) {
	ds := NewRegionDataSource()
	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	expected := []string{"id", "name", "description", "class", "status", "city", "state", "country", "latitude", "longitude"}
	for _, name := range expected {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("expected attribute %q in schema", name)
		}
	}

	idAttr := resp.Schema.Attributes["id"]
	if !idAttr.IsRequired() {
		t.Error("id should be required")
	}
}

func TestRegionDataSource_Configure(t *testing.T) {
	c := client.New("http://localhost", "tok", "org")
	ds := NewRegionDataSource().(*regionDataSource)
	req := datasource.ConfigureRequest{}
	req.ProviderData = c
	resp := &datasource.ConfigureResponse{}
	ds.Configure(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics)
	}
}

func TestUnwrapRegion_FlatResponse(t *testing.T) {
	flat := json.RawMessage(`{"id":"helsinki-1","name":"Helsinki, Finland"}`)
	raw, err := unwrapRegion(flat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw["id"] != "helsinki-1" {
		t.Fatalf("expected helsinki-1, got %v", raw["id"])
	}
}

func TestUnwrapRegion_CollectionWrapped(t *testing.T) {
	wrapped := json.RawMessage(`{"count":1,"regions":[{"id":"virginia-1","name":"Virginia, US"}]}`)
	raw, err := unwrapRegion(wrapped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw["id"] != "virginia-1" {
		t.Fatalf("expected virginia-1, got %v", raw["id"])
	}
}

func TestRawToRegionModel(t *testing.T) {
	raw := map[string]any{
		"id":          "helsinki-1",
		"name":        "Helsinki, Finland",
		"description": "Helsinki",
		"class":       "standard",
		"status":      "active",
		"location": map[string]any{
			"city":      "Helsinki",
			"state":     "Uusimaa",
			"country":   "Finland",
			"latitude":  60.17,
			"longitude": 24.94,
		},
	}

	m := rawToRegionModel(raw)
	if m.ID.ValueString() != "helsinki-1" {
		t.Errorf("ID = %q, want helsinki-1", m.ID.ValueString())
	}
	if m.Name.ValueString() != "Helsinki, Finland" {
		t.Errorf("Name = %q, want 'Helsinki, Finland'", m.Name.ValueString())
	}
	if m.Status.ValueString() != "active" {
		t.Errorf("Status = %q, want active", m.Status.ValueString())
	}
	if m.City.ValueString() != "Helsinki" {
		t.Errorf("City = %q, want Helsinki", m.City.ValueString())
	}
	if m.Latitude.ValueFloat64() != 60.17 {
		t.Errorf("Latitude = %f, want 60.17", m.Latitude.ValueFloat64())
	}
}

func TestRawToRegionModel_NoLocation(t *testing.T) {
	raw := map[string]any{
		"id":     "test-1",
		"name":   "Test",
		"status": "active",
	}
	m := rawToRegionModel(raw)
	if m.ID.ValueString() != "test-1" {
		t.Errorf("ID = %q, want test-1", m.ID.ValueString())
	}
	if !m.City.IsNull() {
		t.Error("City should be null when no location")
	}
	if !m.Latitude.IsNull() {
		t.Error("Latitude should be null when no location")
	}
}

func TestRegionsDataSource_Metadata(t *testing.T) {
	ds := NewRegionsDataSource()
	req := datasource.MetadataRequest{ProviderTypeName: "bahriya"}
	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), req, resp)
	if resp.TypeName != "bahriya_regions" {
		t.Fatalf("expected bahriya_regions, got %s", resp.TypeName)
	}
}

func TestRegionsDataSource_Schema(t *testing.T) {
	ds := NewRegionsDataSource()
	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), req, resp)

	if _, ok := resp.Schema.Attributes["status_filter"]; !ok {
		t.Error("expected status_filter attribute")
	}
	if _, ok := resp.Schema.Attributes["regions"]; !ok {
		t.Error("expected regions attribute")
	}
}

func TestRegionObjectAttrTypes(t *testing.T) {
	attrTypes := regionObjectAttrTypes()
	expected := []string{"id", "name", "description", "class", "status", "city", "state", "country", "latitude", "longitude"}
	for _, name := range expected {
		if _, ok := attrTypes[name]; !ok {
			t.Errorf("expected attr type for %q", name)
		}
	}

	if attrTypes["latitude"] != types.Float64Type {
		t.Error("latitude should be Float64Type")
	}
	if attrTypes["id"] != types.StringType {
		t.Error("id should be StringType")
	}
}
