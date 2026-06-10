package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(handler http.HandlerFunc) (*httptest.Server, *Client) {
	srv := httptest.NewServer(handler)
	c := New(srv.URL, "test-token", "org-123")
	return srv, c
}

func TestDo_ParsesEnvelope(t *testing.T) {
	srv, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"code":   200,
			"status": "ok",
			"data":   map[string]any{"id": "abc-123", "name": "test"},
		})
	})
	defer srv.Close()

	data, status, err := c.Do(context.Background(), http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Fatalf("expected status 200, got %d", status)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if parsed["id"] != "abc-123" {
		t.Fatalf("expected id abc-123, got %v", parsed["id"])
	}
}

func TestDo_SetsAuthHeader(t *testing.T) {
	var gotAuth string
	srv, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{"code": 200, "status": "ok", "data": nil})
	})
	defer srv.Close()

	_, _, _ = c.Do(context.Background(), http.MethodGet, "/auth-check", nil)
	if gotAuth != "Bearer test-token" {
		t.Fatalf("expected Bearer test-token, got %q", gotAuth)
	}
}

func TestDo_MarshalsBody(t *testing.T) {
	var gotBody map[string]any
	srv, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"code": 201, "status": "created", "data": "new-id"})
	})
	defer srv.Close()

	body := map[string]any{"handle": "test", "name": "Test"}
	_, status, err := c.Do(context.Background(), http.MethodPost, "/create", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 201 {
		t.Fatalf("expected 201, got %d", status)
	}
	if gotBody["handle"] != "test" {
		t.Fatalf("expected handle=test, got %v", gotBody["handle"])
	}
}

func TestDo_Returns4xxAsAPIError(t *testing.T) {
	srv, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		json.NewEncoder(w).Encode(map[string]any{
			"code":   422,
			"status": "error",
			"data":   "validation failed",
		})
	})
	defer srv.Close()

	_, status, err := c.Do(context.Background(), http.MethodPost, "/fail", nil)
	if status != 422 {
		t.Fatalf("expected 422, got %d", status)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 422 {
		t.Fatalf("expected APIError.StatusCode 422, got %d", apiErr.StatusCode)
	}
}

func TestDo_RetriesOn5xx(t *testing.T) {
	calls := 0
	srv, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(503)
			json.NewEncoder(w).Encode(map[string]any{"code": 503, "status": "error", "data": nil})
			return
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{"code": 200, "status": "ok", "data": "recovered"})
	})
	defer srv.Close()

	c.httpClient.Timeout = 0 // disable timeout for test

	_, status, err := c.Do(context.Background(), http.MethodGet, "/retry", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Fatalf("expected 200 after retry, got %d", status)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
}

func TestDo_NoRetryOnPOST(t *testing.T) {
	calls := 0
	srv, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(503)
		json.NewEncoder(w).Encode(map[string]any{"code": 503, "status": "error", "data": nil})
	})
	defer srv.Close()

	_, _, _ = c.Do(context.Background(), http.MethodPost, "/no-retry", nil)
	if calls != 1 {
		t.Fatalf("expected 1 attempt for POST, got %d", calls)
	}
}

func TestHandleExists_ReturnsTrue(t *testing.T) {
	srv, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/organisations/org-123/secret/my-secret" {
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]any{"code": 200, "status": "ok", "data": map[string]any{"id": "s-1"}})
			return
		}
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]any{"code": 404, "status": "not_found", "data": nil})
	})
	defer srv.Close()

	exists, err := c.HandleExists(context.Background(), "secret", "my-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected true, got false")
	}
}

func TestHandleExists_ReturnsFalse(t *testing.T) {
	srv, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]any{"code": 404, "status": "not_found", "data": nil})
	})
	defer srv.Close()

	exists, err := c.HandleExists(context.Background(), "secret", "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected false, got true")
	}
}

func TestOrganisationID(t *testing.T) {
	c := New("http://localhost", "tok", "org-abc")
	if c.OrganisationID() != "org-abc" {
		t.Fatalf("expected org-abc, got %s", c.OrganisationID())
	}
}

func TestBaseURL(t *testing.T) {
	c := New("http://localhost:8080/v1", "tok", "org")
	if c.BaseURL() != "http://localhost:8080/v1" {
		t.Fatalf("expected http://localhost:8080/v1, got %s", c.BaseURL())
	}
}
