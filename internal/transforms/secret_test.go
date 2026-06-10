package transforms

import (
	"reflect"
	"testing"
)

func TestSecretsToAPI_SortedAndStable(t *testing.T) {
	in := map[string]string{
		"db-password": "DATABASE_PASSWORD",
		"api-key":     "API_KEY",
	}
	got := SecretsToAPI(in)
	want := []map[string]string{
		{"secret": "api-key", "name": "API_KEY"},
		{"secret": "db-password", "name": "DATABASE_PASSWORD"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSecretsFromAPI_RoundTrip(t *testing.T) {
	api := []map[string]any{
		{"secret": "db-password", "name": "DATABASE_PASSWORD"},
		{"secret": "api-key", "name": "API_KEY"},
	}
	got := SecretsFromAPI(api)
	want := map[string]string{
		"db-password": "DATABASE_PASSWORD",
		"api-key":     "API_KEY",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSecretsFromAPI_SkipsMalformedEntries(t *testing.T) {
	api := []map[string]any{
		{"secret": "db-password", "name": "DATABASE_PASSWORD"},
		{"name": "MISSING_SECRET_KEY"},
		{"secret": "no-name"},
	}
	got := SecretsFromAPI(api)
	want := map[string]string{"db-password": "DATABASE_PASSWORD"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
