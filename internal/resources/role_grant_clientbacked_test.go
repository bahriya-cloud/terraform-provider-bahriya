package resources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
)

// envelope wraps a data payload the way the Bahriya API does.
func writeEnvelope(w http.ResponseWriter, code int, data any) {
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "status": "ok", "data": data})
}

// role.fetch parses a single Role envelope into the model.
func TestRoleFetchParsesRole(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(w, 200, map[string]any{
			"id":       "role-1",
			"handle":   "deployer",
			"name":     "Deployer",
			"issystem": false,
			"permissions": []any{
				map[string]any{"level": "project", "resource": "deployables_container_http", "permission": "create"},
			},
		})
	}))
	defer srv.Close()

	r := &roleResource{client: client.New(srv.URL, "tok", "org-123")}
	m, diags := r.fetch(context.Background(), "role-1")
	if diags.HasError() {
		t.Fatalf("fetch diags: %v", diags)
	}
	if m.Handle.ValueString() != "deployer" || m.Name.ValueString() != "Deployer" {
		t.Errorf("model = %+v", m)
	}
	if len(m.Permissions.Elements()) != 1 {
		t.Errorf("permissions len = %d", len(m.Permissions.Elements()))
	}
}

// grant.read reconstructs one member's grants from the instance-wide list,
// filtering by touser and collecting the backing grant ids.
func TestResourceGrantReadFiltersByUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(w, 200, map[string]any{
			"count": 3,
			"grants": []any{
				map[string]any{"id": "g1", "touser": "user-a", "permission": "read"},
				map[string]any{"id": "g2", "touser": "user-a", "permission": "update"},
				map[string]any{"id": "g3", "touser": "user-b", "permission": "read"},
			},
		})
	}))
	defer srv.Close()

	r := &resourceGrantResource{client: client.New(srv.URL, "tok", "org-123")}
	m, diags := r.read(context.Background(), "user-a", "attachables_registries", "inst-1")
	if diags.HasError() {
		t.Fatalf("read diags: %v", diags)
	}
	if len(m.Permissions.Elements()) != 2 {
		t.Fatalf("expected 2 permissions for user-a, got %d", len(m.Permissions.Elements()))
	}
	if len(m.GrantIds.Elements()) != 2 {
		t.Fatalf("expected 2 grant ids for user-a, got %d", len(m.GrantIds.Elements()))
	}
	// user-b's grant (g3) must not leak in.
	for _, el := range m.GrantIds.Elements() {
		if s, ok := el.(types.String); ok && s.ValueString() == "g3" {
			t.Errorf("user-b grant leaked into user-a state")
		}
	}
	if m.ID.ValueString() != "attachables_registries|inst-1|user-a" {
		t.Errorf("synthetic id = %q", m.ID.ValueString())
	}
}

// A member with no grants on the instance reads as not-found (→ resource removed).
func TestResourceGrantReadNotFoundForUnsharedUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(w, 200, map[string]any{"count": 1, "grants": []any{
			map[string]any{"id": "g1", "touser": "someone-else", "permission": "read"},
		}})
	}))
	defer srv.Close()

	r := &resourceGrantResource{client: client.New(srv.URL, "tok", "org-123")}
	_, diags := r.read(context.Background(), "user-a", "attachables_registries", "inst-1")
	if !diagsContainNotFound(diags) {
		t.Errorf("expected not_found for an unshared user, got %v", diags)
	}
}
