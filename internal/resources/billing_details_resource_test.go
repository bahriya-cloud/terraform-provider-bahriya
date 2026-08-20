package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// bahriya_billing_details is a hand-written singleton; these tests pin the
// alias <-> API field mapping in both directions and the schema contract, so
// a rename on either side fails here before it fails against the API.

func TestApiToBillingDetailsModelFields(t *testing.T) {
	raw := map[string]any{
		"organisationid":     "11111111-1111-1111-1111-111111111111",
		"name":               "Acme",
		"legalname":          "Acme Trading LLC",
		"email":              "accounts@acme.example",
		"billingentitytype":  "company",
		"addressline1":       "Office 402",
		"addressline2":       nil,
		"addresscity":        "Dubai",
		"addressstate":       "Dubai",
		"addresspostcode":    nil,
		"addresscountry":     "AE",
		"taxid":              "100123456789003",
		"taxidtype":          "trn",
		"registrationnumber": "1234567",
		"billingreference":   "PO-4471",
		"complete":           true,
	}

	m := apiToBillingDetailsModel(raw)

	if m.ID.ValueString() != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("id = %q", m.ID.ValueString())
	}
	if m.LegalName.ValueString() != "Acme Trading LLC" {
		t.Errorf("legal_name = %q", m.LegalName.ValueString())
	}
	if m.EntityType.ValueString() != "company" {
		t.Errorf("entity_type = %q", m.EntityType.ValueString())
	}
	if m.Country.ValueString() != "AE" {
		t.Errorf("country = %q", m.Country.ValueString())
	}
	if !m.AddressLine2.IsNull() {
		t.Errorf("address_line2 should map null to null")
	}
	if m.TaxIDType.ValueString() != "trn" {
		t.Errorf("tax_id_type = %q", m.TaxIDType.ValueString())
	}
	if m.BillingReference.ValueString() != "PO-4471" {
		t.Errorf("billing_reference = %q", m.BillingReference.ValueString())
	}
	if !m.Complete.ValueBool() {
		t.Errorf("complete should be true")
	}
}

func TestBillingDetailsFieldMapCoversEveryWritableAlias(t *testing.T) {
	want := map[string]bool{
		"legalname": false, "billingentitytype": false,
		"addressline1": false, "addressline2": false, "addresscity": false,
		"addressstate": false, "addresspostcode": false, "addresscountry": false,
		"taxid": false, "taxidtype": false, "registrationnumber": false,
		"billingreference": false,
	}
	for _, f := range billingDetailsFieldMap {
		if _, ok := want[f.api]; !ok {
			t.Errorf("unexpected API field %q in map", f.api)
			continue
		}
		want[f.api] = true
	}
	for api, seen := range want {
		if !seen {
			t.Errorf("API field %q missing from billingDetailsFieldMap", api)
		}
	}
}

func TestBillingDetailsFieldMapReadsThePlan(t *testing.T) {
	plan := billingDetailsModel{
		LegalName:  types.StringValue("Acme Trading LLC"),
		Country:    types.StringValue("AE"),
		TaxID:      types.StringNull(),
		EntityType: types.StringValue("company"),
	}

	got := map[string]any{}
	for _, f := range billingDetailsFieldMap {
		got[f.api] = stringOrNil(f.get(&plan))
	}

	if got["legalname"] != "Acme Trading LLC" {
		t.Errorf("legalname = %v", got["legalname"])
	}
	if got["addresscountry"] != "AE" {
		t.Errorf("addresscountry = %v", got["addresscountry"])
	}
	if got["taxid"] != nil {
		t.Errorf("null plan value must serialise as nil (explicit clear), got %v", got["taxid"])
	}
	if got["billingentitytype"] != "company" {
		t.Errorf("billingentitytype = %v", got["billingentitytype"])
	}
}

func TestBillingDetailsSchemaContract(t *testing.T) {
	r := &billingDetailsResource{}
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)

	attrs := resp.Schema.Attributes
	for _, name := range []string{
		"id", "legal_name", "email", "entity_type",
		"address_line1", "address_line2", "city", "state", "postcode", "country",
		"tax_id", "tax_id_type", "registration_number", "billing_reference", "complete",
	} {
		if _, ok := attrs[name]; !ok {
			t.Errorf("schema missing attribute %q", name)
		}
	}
	if len(attrs) != 15 {
		t.Errorf("schema has %d attributes, want 15 — update the docs when adding one", len(attrs))
	}
}
