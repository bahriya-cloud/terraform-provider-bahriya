package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
)

func TestProvider_Metadata(t *testing.T) {
	p := &bahriyaProvider{version: "1.0.0"}
	req := provider.MetadataRequest{}
	resp := &provider.MetadataResponse{}
	p.Metadata(context.Background(), req, resp)

	if resp.TypeName != "bahriya" {
		t.Fatalf("expected type name bahriya, got %s", resp.TypeName)
	}
	if resp.Version != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %s", resp.Version)
	}
}

func TestProvider_Schema(t *testing.T) {
	p := &bahriyaProvider{}
	req := provider.SchemaRequest{}
	resp := &provider.SchemaResponse{}
	p.Schema(context.Background(), req, resp)

	expected := []string{"token", "base_url", "organisation_id"}
	for _, name := range expected {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("expected attribute %q in provider schema", name)
		}
	}

	tokenAttr := resp.Schema.Attributes["token"]
	if !tokenAttr.IsSensitive() {
		t.Error("token should be sensitive")
	}
}

func TestProvider_Resources_NotEmpty(t *testing.T) {
	p := &bahriyaProvider{}
	resources := p.Resources(context.Background())
	if len(resources) == 0 {
		t.Fatal("expected at least one resource")
	}
	if len(resources) != 32 {
		t.Fatalf("expected 32 resources, got %d", len(resources))
	}
}

func TestProvider_DataSources_NotEmpty(t *testing.T) {
	// The expected SET, not a count. A bare count fails on every addition with a message that says
	// only that the number moved, which tells the next person nothing about whether the change was
	// intended — so the reflex is to bump the number, and the check stops meaning anything.
	want := map[string]bool{
		"bahriya_organisation":  true,
		"bahriya_region":        true,
		"bahriya_regions":       true,
		"bahriya_project_quota": true,
	}

	p := &bahriyaProvider{}
	dataSources := p.DataSources(context.Background())
	if len(dataSources) == 0 {
		t.Fatal("expected at least one data source")
	}

	got := map[string]bool{}
	for _, newDataSource := range dataSources {
		var resp datasource.MetadataResponse
		newDataSource().Metadata(
			context.Background(),
			datasource.MetadataRequest{ProviderTypeName: "bahriya"},
			&resp,
		)
		got[resp.TypeName] = true
	}

	for name := range want {
		if !got[name] {
			t.Errorf("data source %q is registered nowhere — add it to datasources.All()", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("data source %q is new; add it to this test and to docs/data-sources/", name)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		want string
	}{
		{"first", []string{"a", "b"}, "a"},
		{"second", []string{"", "b"}, "b"},
		{"none", []string{"", ""}, ""},
		{"third", []string{"", "", "c"}, "c"},
		{"single", []string{"x"}, "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstNonEmpty(tt.vals...)
			if got != tt.want {
				t.Errorf("firstNonEmpty(%v) = %q, want %q", tt.vals, got, tt.want)
			}
		})
	}
}
