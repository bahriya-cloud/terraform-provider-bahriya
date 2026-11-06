package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Regenerating reproduces the committed files byte for byte: the output is
// deterministic, and internal/resources is in step with the generator and the
// embedded resource-schema.json (`make verify` checks the same in CI).
func TestGeneratedFilesAreCurrent(t *testing.T) {
	out := t.TempDir()
	if err := run(out); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(kinds)+len(attachments)+1 {
		t.Errorf("generated %d files, want %d", len(entries), len(kinds)+len(attachments)+1)
	}
	for _, e := range entries {
		got, _ := os.ReadFile(filepath.Join(out, e.Name()))
		committed, err := os.ReadFile(filepath.Join("..", "..", "internal", "resources", e.Name()))
		if err != nil {
			t.Errorf("%s is not committed: %v", e.Name(), err)
			continue
		}
		if !bytes.Equal(got, committed) {
			t.Errorf("internal/resources/%s is stale: run `make generate`", e.Name())
		}
	}
}

func TestLowerFirst(t *testing.T) {
	for in, want := range map[string]string{"Project": "project", "TLSBundle": "tlsBundle", "SSHKeypair": "sshKeypair", "YAMLConfig": "yamlConfig", "X509Cert": "x509Cert"} {
		if got := lowerFirst(in); got != want {
			t.Errorf("lowerFirst(%q) = %q, want %q", in, got, want)
		}
	}
}
