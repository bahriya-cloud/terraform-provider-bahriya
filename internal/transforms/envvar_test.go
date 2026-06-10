package transforms

import (
	"reflect"
	"testing"
)

func TestEnvVarsToAPI_SortedAndStable(t *testing.T) {
	in := map[string]string{
		"DATABASE_URL": "postgres://x",
		"API_KEY":      "secret",
		"PORT":         "8080",
	}
	got := EnvVarsToAPI(in)
	want := []map[string]string{
		{"name": "API_KEY", "value": "secret"},
		{"name": "DATABASE_URL", "value": "postgres://x"},
		{"name": "PORT", "value": "8080"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestEnvVarsToAPI_Empty(t *testing.T) {
	got := EnvVarsToAPI(map[string]string{})
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestEnvVarsFromAPI_RoundTrip(t *testing.T) {
	api := []map[string]any{
		{"name": "API_KEY", "value": "secret"},
		{"name": "PORT", "value": "8080"},
	}
	got := EnvVarsFromAPI(api)
	want := map[string]string{"API_KEY": "secret", "PORT": "8080"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestEnvVarsFromAPI_SkipsMalformedEntries(t *testing.T) {
	api := []map[string]any{
		{"name": "API_KEY", "value": "secret"},
		{"name": "MISSING_VALUE"},
		{"value": "no name here"},
		{"name": 123, "value": "wrong type"},
	}
	got := EnvVarsFromAPI(api)
	want := map[string]string{"API_KEY": "secret"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
