package engine

import (
	"context"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/bmap"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// Every kind builds a valid schema: no reserved root name, every type
// supported, and the framework's own validation passes.
func TestEveryKindBuildsAValidSchema(t *testing.T) {
	for _, name := range bmap.Embedded().Kinds() {
		k := bmap.Embedded().Kind(name)
		s, err := BuildSchema(k, "")
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if diags := s.ValidateImplementation(context.Background()); diags.HasError() {
			t.Errorf("%s: %v", name, diags)
		}
	}
}

// A reserved root name is refused, naming the field (negative control for the
// guard above: the container group `lifecycle` hit this in the #2244 spike).
func TestReservedRootNameIsRefused(t *testing.T) {
	k := bmap.Embedded().Kind("secret")
	clone := *k
	clone.Fields = append([]*bmap.Field{}, k.Fields...)
	f := *clone.Fields[0]
	f.Path = "lifecycle.thing"
	clone.Fields[0] = &f
	if _, err := BuildSchema(&clone, ""); err == nil {
		t.Fatal("a field under `lifecycle` built a schema")
	}
}

// Groups are nested attributes, list items nested lists, env maps maps; a
// required field without a default is Required and makes its group Required.
func TestContainerShape(t *testing.T) {
	s, err := BuildSchema(bmap.Embedded().Kind("container"), "")
	if err != nil {
		t.Fatal(err)
	}
	workload, ok := s.Attributes["workload"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("workload is %T, want SingleNestedAttribute", s.Attributes["workload"])
	}
	if !workload.Required {
		t.Error("workload holds the required image, so it must be Required")
	}
	if _, ok := workload.Attributes["image"].(schema.StringAttribute); !ok {
		t.Errorf("workload.image is %T", workload.Attributes["image"])
	}
	if _, ok := s.Attributes["init_jobs"].(schema.ListNestedAttribute); !ok {
		t.Errorf("init_jobs is %T, want ListNestedAttribute", s.Attributes["init_jobs"])
	}
	if id, ok := s.Attributes["id"].(schema.StringAttribute); !ok || !id.Computed || id.Optional {
		t.Errorf("id must be Computed only, got %#v", s.Attributes["id"])
	}
}
