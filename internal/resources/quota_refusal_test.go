package resources

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// A create refused on project resource limits reads as the structured
// message (which ceiling, where, by how much), not "Create failed" with the
// raw JSON: the client returns an error for every 4xx, so the 409 body must be
// examined before that error is reported.
func TestCreateQuotaRefusalIsReadable(t *testing.T) {
	const org = "00000000-0000-4000-8000-00000000000f"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet: // handle-exists: free
			w.WriteHeader(404)
			_, _ = io.WriteString(w, `{"code":404,"status":"NOT_FOUND","data":"Not found."}`)
		default:
			w.WriteHeader(409)
			_, _ = io.WriteString(w, `{"code":409,"status":"CONFLICT","data":{"blocked":true,"message":"This change exceeds the resource limits for project \"shop\".","breaches":[{"region":"helsinki-1","axis":"requests.memory","label":"reserved memory","project":"shop","used":"0B","requested":"1472Mi","available":"1G","headroom":"1G","blamed":["orders"]}],"warnings":[],"attainablereplicas":{}}}`)
		}
	}))
	defer server.Close()

	r := &memcachedResource{client: client.New(server.URL, "token", org)}
	var sresp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &sresp)
	s := sresp.Schema
	objType := s.Type().TerraformType(context.Background()).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for name, typ := range objType.AttributeTypes {
		values[name] = tftypes.NewValue(typ, nil)
	}
	values["handle"] = tftypes.NewValue(tftypes.String, "orders")
	values["name"] = tftypes.NewValue(tftypes.String, "Orders")
	plan := tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(objType, values)}

	resp := resource.CreateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(objType, nil)}}
	r.Create(context.Background(), resource.CreateRequest{Plan: plan}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("a refused create must be an error")
	}
	got := resp.Diagnostics.Errors()[0]
	if got.Summary() != "Project resource limits exceeded" {
		t.Errorf("summary %q, want \"Project resource limits exceeded\" (detail: %s)", got.Summary(), got.Detail())
	}
	if !strings.Contains(got.Detail(), "reserved memory") {
		t.Errorf("the detail must name the ceiling, got %q", got.Detail())
	}
}
