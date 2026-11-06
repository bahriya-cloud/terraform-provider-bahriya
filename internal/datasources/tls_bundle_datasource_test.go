package datasources

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func TestTlsBundleDataSource_Metadata(t *testing.T) {
	ds := NewTlsBundleDataSource()
	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "bahriya"}, resp)
	if resp.TypeName != "bahriya_tls_bundle" {
		t.Fatalf("expected bahriya_tls_bundle, got %s", resp.TypeName)
	}
}

func TestTlsBundleDataSource_Schema(t *testing.T) {
	ds := NewTlsBundleDataSource()
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, resp)

	handle, ok := resp.Schema.Attributes["handle"].(schema.StringAttribute)
	if !ok || !handle.Required {
		t.Fatal("handle must be a required string attribute")
	}
	for _, name := range []string{"id", "name", "ca", "cert", "subject", "issuer", "fingerprint", "expires", "managed"} {
		attr, ok := resp.Schema.Attributes[name]
		if !ok {
			t.Errorf("expected attribute %q in schema", name)
			continue
		}
		if !attr.IsComputed() {
			t.Errorf("attribute %q must be computed", name)
		}
	}
	if _, ok := resp.Schema.Attributes["key"]; ok {
		t.Error("the private key must never be an attribute")
	}
}

func TestTlsBundleIDForHandle(t *testing.T) {
	list := json.RawMessage(`{"tlsbundles":[{"id":"a1","handle":"orders-db-tls"},{"id":"b2","handle":"my-cert"}],"pagination":{"page":1}}`)

	id, err := tlsBundleIDForHandle(list, "orders-db-tls")
	if err != nil || id != "a1" {
		t.Fatalf("expected a1, got %q (%v)", id, err)
	}
	if _, err := tlsBundleIDForHandle(list, "nope"); err == nil {
		t.Fatal("expected an error for an unknown handle")
	}
	if _, err := tlsBundleIDForHandle(json.RawMessage(`{}`), "orders-db-tls"); err == nil {
		t.Fatal("expected an error for an empty list")
	}
}

func TestTlsBundleStateFromDetail(t *testing.T) {
	detail := json.RawMessage(`{
		"id": "a1", "name": "Orders DB TLS", "handle": "orders-db-tls",
		"managedbyresourceid": "m-1", "managedbyresourcetype": "mysql",
		"subject": "CN=orders-db", "issuer": "CN=orders-db CA", "fingerprint": "AB:CD",
		"notafter": {"date": "2029-10-21 00:00:00.000000", "timezone_type": 3, "timezone": "UTC"},
		"ca": "-----BEGIN CERTIFICATE-----\nCA\n-----END CERTIFICATE-----\n",
		"cert": "-----BEGIN CERTIFICATE-----\nLEAF\n-----END CERTIFICATE-----\n",
		"key": "value-hidden-for-your-own-good"
	}`)

	state, err := tlsBundleStateFromDetail(detail, "orders-db-tls")
	if err != nil {
		t.Fatal(err)
	}
	if state.ID.ValueString() != "a1" || state.Handle.ValueString() != "orders-db-tls" {
		t.Errorf("identity not mapped: %+v", state)
	}
	if state.CA.ValueString() != "-----BEGIN CERTIFICATE-----\nCA\n-----END CERTIFICATE-----\n" {
		t.Errorf("ca not mapped verbatim: %q", state.CA.ValueString())
	}
	if state.Cert.ValueString() == "" || state.Subject.ValueString() != "CN=orders-db" {
		t.Errorf("certificate fields not mapped: %+v", state)
	}
	if state.NotAfter.ValueString() != "2029-10-21 00:00:00.000000" {
		t.Errorf("notafter object not read: %q", state.NotAfter.ValueString())
	}
	if !state.Managed.ValueBool() {
		t.Error("a bundle with managedbyresourceid is managed")
	}

	customer, err := tlsBundleStateFromDetail(json.RawMessage(`{"id":"b2","managedbyresourceid":null,"notafter":"2027-01-01T00:00:00Z"}`), "my-cert")
	if err != nil {
		t.Fatal(err)
	}
	if customer.Managed.ValueBool() {
		t.Error("a customer bundle is not managed")
	}
	if customer.NotAfter.ValueString() != "2027-01-01T00:00:00Z" {
		t.Errorf("string notafter not read: %q", customer.NotAfter.ValueString())
	}
	if !customer.CA.IsNull() {
		t.Error("a missing ca is null")
	}
}
