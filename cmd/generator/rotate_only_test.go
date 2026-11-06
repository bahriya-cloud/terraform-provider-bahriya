package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const rotateSpec = `openapi: 3.0.0
components:
  schemas:
    TlsBundle:
      type: object
      required: [handle, name, cert]
      properties:
        handle: {type: string}
        name: {type: string}
        cert: {type: string}
`

func writeSpec(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.yaml")
	if err := os.WriteFile(path, []byte(rotateSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// #2620: material listed in RotateOnlyFields is marked, and nothing else is.
func TestParseResource_RotateOnlyFields(t *testing.T) {
	r, err := parseResource(writeSpec(t), resourceDescriptor{Name: "tls_bundle", GoName: "TLSBundle", SchemaName: "TlsBundle", RotateOnlyFields: []string{"cert"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range r.Attributes {
		if a.RotateOnly != (a.APIName == "cert") {
			t.Errorf("%s: RotateOnly = %v", a.APIName, a.RotateOnly)
		}
	}
}

func TestParseResource_RotateOnlyFieldMustExist(t *testing.T) {
	_, err := parseResource(writeSpec(t), resourceDescriptor{Name: "tls_bundle", GoName: "TLSBundle", SchemaName: "TlsBundle", RotateOnlyFields: []string{"certificate"}})
	if err == nil || !strings.Contains(err.Error(), `rotate-only field "certificate"`) {
		t.Fatalf("a rotate-only field that is not a property must be refused, got %v", err)
	}
}

// The nine kinds whose update only renames carry their material as rotate-only.
func TestRotateOnlyFieldsCoverTheRenameOnlyKinds(t *testing.T) {
	want := map[string]string{
		"tls_bundle": "ca,cert,key", "x509_cert": "cert", "gpg_keypair": "publickey,privatekey",
		"ssh_keypair": "publickey,privatekey", "encryption_key": "key", "env_file": "content",
		"yaml_config": "content", "json_config": "content", "plain_config": "content",
	}
	for name, d := range descriptors {
		got := strings.Join(d.RotateOnlyFields, ",")
		if got != want[name] {
			t.Errorf("%s: RotateOnlyFields = %q, want %q", name, got, want[name])
		}
		immutable := strings.Join(d.ImmutableFields, ",")
		if wantImmutable := map[string]string{"encryption_key": "format"}[name]; immutable != wantImmutable {
			t.Errorf("%s: ImmutableFields = %q, want %q", name, immutable, wantImmutable)
		}
	}
}
