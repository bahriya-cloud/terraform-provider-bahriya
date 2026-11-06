package resources

import (
	"context"
	"reflect"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/engine"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// The generated schemas are the engine's, rendered as source. If they differ,
// resource-schema.json changed without `make generate`, or the renderer and
// the runtime builder disagree; either way the generated files are stale.
func TestGeneratedSchemasMatchTheEngine(t *testing.T) {
	served := 0
	for _, newResource := range All() {
		r, ok := newResource().(*engine.Resource)
		if !ok {
			continue // attachments and hand-written resources
		}
		served++
		var meta resource.MetadataResponse
		r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "bahriya"}, &meta)
		name := meta.TypeName[len("bahriya_"):]

		var got resource.SchemaResponse
		r.Schema(context.Background(), resource.SchemaRequest{}, &got)
		want, err := engine.KindSchema(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(got.Schema, want) {
			t.Errorf("%s: the generated schema differs from the engine's: run `make generate`", name)
		}
	}
	if served != len(engine.Descriptors) {
		t.Errorf("the registry serves %d engine kinds, the engine describes %d", served, len(engine.Descriptors))
	}
}
