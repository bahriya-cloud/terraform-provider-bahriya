package engine

import (
	"sort"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/bmap"
)

// sensitivePaths lists every Sensitive attribute of a kind as "kind.path",
// nested attributes joined with dots.
func sensitivePaths(t *testing.T, k *bmap.Kind) []string {
	t.Helper()
	attrs, err := Attributes(k)
	if err != nil {
		t.Fatalf("%s: %v", k.Name, err)
	}
	var out []string
	var walk func(prefix string, attrs []*Attr)
	walk = func(prefix string, attrs []*Attr) {
		for _, a := range attrs {
			if a.Sensitive {
				out = append(out, prefix+a.Name)
			}
			walk(prefix+a.Name+".", a.Children)
		}
	}
	walk(k.Name+".", attrs)
	sort.Strings(out)
	return out
}

// Plans hide exactly these values (#2646). Sensitivity comes from x-sensitive
// or writeOnly, never from x-masked: a masked field is one whose read may be
// a placeholder, and public certificates and config content are masked in
// list reads without being secret. The direction that leaks is a secret losing
// Sensitive, so the set is pinned by name: a field leaving it, or a new one
// joining it, fails here until this list is changed on purpose.
var wantSensitive = []string{
	"container.security.basic_auth.password",
	"encryption_key.key",
	"env_file.content",
	"gpg_keypair.private_key",
	"gpg_keypair.public_key",
	"pathrule.security.basic_auth.password",
	"registry.password",
	"secret.value",
	"ssh_keypair.private_key",
	"ssh_keypair.public_key",
	"tls_bundle.key",
	"valkey.security.password",
}

func TestExactlyTheSecretFieldsAreSensitive(t *testing.T) {
	var got []string
	for _, name := range bmap.Embedded().Kinds() {
		got = append(got, sensitivePaths(t, bmap.Embedded().Kind(name))...)
	}
	sort.Strings(got)
	want := append([]string{}, wantSensitive...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("Sensitive attributes\n got: %q\nwant: %q", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("Sensitive attributes\n got: %q\nwant: %q", got, want)
		}
	}
}

// Negative control for the guard above: a secret value without x-sensitive is
// no longer Sensitive, even though it is still masked, so the pinned list
// would catch it.
func TestASecretWithoutXSensitiveLosesSensitive(t *testing.T) {
	k := bmap.Embedded().Kind("secret")
	clone := *k
	clone.Fields = append([]*bmap.Field{}, k.Fields...)
	for i, f := range clone.Fields {
		if f.Name == "value" {
			copied := *f
			desc := *copied.Desc
			desc.Sensitive = false
			copied.Desc = &desc
			clone.Fields[i] = &copied
			if desc.Masked == "" {
				t.Fatal("secret.value is not masked; this control needs a masked field")
			}
		}
	}
	for _, p := range sensitivePaths(t, &clone) {
		if p == "secret.value" {
			t.Fatal("secret.value stayed Sensitive without x-sensitive: masked must not imply Sensitive")
		}
	}
	for _, p := range sensitivePaths(t, k) {
		if p == "secret.value" {
			return
		}
	}
	t.Fatal("the unmodified secret.value is not Sensitive")
}
