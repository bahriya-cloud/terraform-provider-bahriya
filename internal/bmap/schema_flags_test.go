package bmap

import "testing"

// x-rotate-only and x-immutable reach the descriptor: the engine rotates the
// first and refuses a change to the second, so losing either in parsing would
// silently turn a rotation into an ignored update.
func TestRotateOnlyAndImmutableAreParsed(t *testing.T) {
	cases := []struct {
		kind, field           string
		rotateOnly, immutable bool
	}{
		{"tls_bundle", "cert", true, false},
		{"tls_bundle", "key", true, false},
		{"encryption_key", "key", true, false},
		{"encryption_key", "format", false, true},
		{"env_file", "content", true, false},
		{"secret", "value", false, false},
	}
	for _, c := range cases {
		k := Embedded().Kind(c.kind)
		if k == nil {
			t.Fatalf("kind %s missing", c.kind)
		}
		f := k.Field(c.field)
		if f == nil {
			t.Fatalf("%s.%s missing", c.kind, c.field)
		}
		if f.Desc.RotateOnly != c.rotateOnly || f.Desc.Immutable != c.immutable {
			t.Errorf("%s.%s: rotateOnly=%v immutable=%v, want %v %v", c.kind, c.field, f.Desc.RotateOnly, f.Desc.Immutable, c.rotateOnly, c.immutable)
		}
	}
}
