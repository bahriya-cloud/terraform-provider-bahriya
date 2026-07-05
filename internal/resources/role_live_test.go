package resources

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
)

// TestRoleLiveLifecycle exercises the real /organisations/{id}/roles endpoints
// (create → read → update → delete) via the client, to catch contract drift the
// mock tests can't. Gated on BAHRIYA_ACC=1 plus BAHRIYA_TOKEN /
// BAHRIYA_ORGANISATION_ID / BAHRIYA_API_URL so it never runs by accident — and
// only against whatever org those creds point at (use localhost dev, not prod).
func TestRoleLiveLifecycle(t *testing.T) {
	if os.Getenv("BAHRIYA_ACC") != "1" {
		t.Skip("set BAHRIYA_ACC=1 (+ BAHRIYA_TOKEN, BAHRIYA_ORGANISATION_ID, BAHRIYA_API_URL) to run the live role test")
	}
	token := os.Getenv("BAHRIYA_TOKEN")
	org := os.Getenv("BAHRIYA_ORGANISATION_ID")
	baseURL := os.Getenv("BAHRIYA_API_URL")
	if token == "" || org == "" || baseURL == "" {
		t.Skip("BAHRIYA_TOKEN, BAHRIYA_ORGANISATION_ID and BAHRIYA_API_URL are required")
	}

	ctx := context.Background()
	r := &roleResource{client: client.New(baseURL, token, org)}

	// Create
	created, status, err := r.client.Do(ctx, http.MethodPost, r.basePath(), map[string]any{
		"name":        "TF Acc Role",
		"description": "created by terraform provider acceptance test",
		"permissions": []map[string]any{
			{"level": "organisation", "resource": "attachables_registries", "permission": "read"},
		},
	})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create role: status=%d err=%v body=%s", status, err, string(created))
	}
	var createdRole map[string]any
	if err := json.Unmarshal(created, &createdRole); err != nil {
		t.Fatalf("decode created role: %v", err)
	}
	id, _ := createdRole["id"].(string)
	if id == "" {
		t.Fatalf("create returned no id: %s", string(created))
	}
	t.Cleanup(func() {
		_, _, _ = r.client.Do(context.Background(), http.MethodDelete, r.basePath()+"/"+id, nil)
	})

	// Read
	model, diags := r.fetch(ctx, id)
	if diags.HasError() {
		t.Fatalf("fetch: %v", diags)
	}
	if model.Name.ValueString() != "TF Acc Role" {
		t.Errorf("name = %q", model.Name.ValueString())
	}
	if model.Issystem.ValueBool() {
		t.Errorf("custom role should not be a system role")
	}
	if len(model.Permissions.Elements()) != 1 {
		t.Errorf("permissions len = %d", len(model.Permissions.Elements()))
	}

	// Update (rename + add a permission)
	_, status, err = r.client.Do(ctx, http.MethodPut, r.basePath()+"/"+id, map[string]any{
		"name": "TF Acc Role Renamed",
		"permissions": []map[string]any{
			{"level": "organisation", "resource": "attachables_registries", "permission": "read"},
			{"level": "organisation", "resource": "attachables_secrets", "permission": "read"},
		},
	})
	if err != nil || status != http.StatusOK {
		t.Fatalf("update role: status=%d err=%v", status, err)
	}
	model, diags = r.fetch(ctx, id)
	if diags.HasError() {
		t.Fatalf("fetch after update: %v", diags)
	}
	if model.Name.ValueString() != "TF Acc Role Renamed" || len(model.Permissions.Elements()) != 2 {
		t.Errorf("after update: name=%q perms=%d", model.Name.ValueString(), len(model.Permissions.Elements()))
	}

	// Delete
	_, status, err = r.client.Do(ctx, http.MethodDelete, r.basePath()+"/"+id, nil)
	if err != nil || status != http.StatusOK {
		t.Fatalf("delete role: status=%d err=%v", status, err)
	}
}
