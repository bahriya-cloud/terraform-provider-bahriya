package resources

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
)

// TestBillingDetailsLiveRoundTrip exercises the real
// /organisations/{id}/billing/details singleton (read → full-replace put →
// read → clear) via the client, to catch contract drift the mock tests
// can't. Gated on BAHRIYA_ACC=1 plus BAHRIYA_TOKEN /
// BAHRIYA_ORGANISATION_ID / BAHRIYA_API_URL so it never runs by accident —
// and only against whatever org those creds point at (use localhost dev,
// not prod). Restores a cleared identity on cleanup.
func TestBillingDetailsLiveRoundTrip(t *testing.T) {
	if os.Getenv("BAHRIYA_ACC") != "1" {
		t.Skip("set BAHRIYA_ACC=1 (+ BAHRIYA_TOKEN, BAHRIYA_ORGANISATION_ID, BAHRIYA_API_URL) to run the live billing details test")
	}
	token := os.Getenv("BAHRIYA_TOKEN")
	org := os.Getenv("BAHRIYA_ORGANISATION_ID")
	baseURL := os.Getenv("BAHRIYA_API_URL")
	if token == "" || org == "" || baseURL == "" {
		t.Skip("BAHRIYA_TOKEN, BAHRIYA_ORGANISATION_ID and BAHRIYA_API_URL are required")
	}

	ctx := context.Background()
	r := &billingDetailsResource{client: client.New(baseURL, token, org)}

	clear := func() {
		body := map[string]any{}
		for _, f := range billingDetailsFieldMap {
			body[f.api] = nil
		}
		_, _, _ = r.client.Do(context.Background(), http.MethodPut, r.path(), body)
	}
	t.Cleanup(clear)

	// Read the singleton — always exists, even when everything is null.
	initial, diags := r.fetch(ctx)
	if diags.HasError() {
		t.Fatalf("initial fetch: %v", diags)
	}
	if initial.ID.ValueString() != org {
		t.Fatalf("id = %q, want the organisation id %q", initial.ID.ValueString(), org)
	}

	// Full-replace put with a complete company identity.
	body := map[string]any{
		"legalname":          "TF Acc Trading LLC",
		"billingentitytype":  "company",
		"addressline1":       "Acceptance Street 1",
		"addressline2":       nil,
		"addresscity":        "Dubai",
		"addressstate":       nil,
		"addresspostcode":    nil,
		"addresscountry":     "ae", // the API upper-cases
		"taxid":              "100123456789003",
		"taxidtype":          "trn",
		"registrationnumber": nil,
		"billingreference":   "TF-ACC-1",
	}
	data, status, err := r.client.Do(ctx, http.MethodPut, r.path(), body)
	if err != nil || status != http.StatusOK {
		t.Fatalf("put: status=%d err=%v body=%s", status, err, string(data))
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decode put response: %v", err)
	}
	m := apiToBillingDetailsModel(raw)
	if m.LegalName.ValueString() != "TF Acc Trading LLC" {
		t.Errorf("legal_name = %q", m.LegalName.ValueString())
	}
	if m.Country.ValueString() != "AE" {
		t.Errorf("country = %q, want normalised AE", m.Country.ValueString())
	}
	if !m.Complete.ValueBool() {
		t.Errorf("complete should be true for a full company identity")
	}

	// A bad country is a 400, not a 500.
	_, status, err = r.client.Do(ctx, http.MethodPut, r.path(), map[string]any{"addresscountry": "XX"})
	if status != http.StatusBadRequest {
		t.Errorf("bad country: status=%d err=%v, want 400", status, err)
	}

	// Clear (the Delete semantics) and confirm the identity is gone.
	clear()
	after, diags := r.fetch(ctx)
	if diags.HasError() {
		t.Fatalf("fetch after clear: %v", diags)
	}
	if !after.LegalName.IsNull() {
		t.Errorf("legal_name should be null after clear, got %q", after.LegalName.ValueString())
	}
	if after.Email.IsNull() {
		t.Errorf("email must survive a clear")
	}
}
