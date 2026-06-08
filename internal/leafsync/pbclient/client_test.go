package pbclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthWithPasswordStoresTokenAndReturnsRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/collections/leaf_nodes/auth-with-password" {
			t.Errorf("unexpected auth path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":  "tok1",
			"record": map[string]any{"id": "leaf1", "domain": "edge-x"},
		})
	}))
	defer ts.Close()

	c := New(ts.URL)
	rec, err := c.AuthWithPassword(context.Background(), "leaf_nodes", "edge01@x", "pw")
	if err != nil {
		t.Fatalf("AuthWithPassword: %v", err)
	}
	if rec["id"] != "leaf1" || rec["domain"] != "edge-x" {
		t.Errorf("unexpected record: %v", rec)
	}
	if c.token != "tok1" {
		t.Errorf("token not stored, got %q", c.token)
	}
}

func TestAuthWithPasswordFailureReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Failed to authenticate."}`))
	}))
	defer ts.Close()

	c := New(ts.URL)
	if _, err := c.AuthWithPassword(context.Background(), "leaf_nodes", "x", "y"); err == nil {
		t.Fatal("expected auth error, got nil")
	}
}

// TestGetReauthenticatesOnce verifies the daemon survives a token expiry: the
// first authed GET returns 401, the client transparently re-authenticates, and
// the retried GET succeeds with the fresh token sent as a raw Authorization header.
func TestGetReauthenticatesOnce(t *testing.T) {
	var authCount, getCount int
	var lastAuthHeader string

	mux := http.NewServeMux()
	mux.HandleFunc("/api/collections/leaf_nodes/auth-with-password", func(w http.ResponseWriter, r *http.Request) {
		authCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":  fmt.Sprintf("tok%d", authCount),
			"record": map[string]any{"id": "leaf1"},
		})
	})
	mux.HandleFunc("/api/collections/things/records", func(w http.ResponseWriter, r *http.Request) {
		getCount++
		if getCount == 1 {
			w.WriteHeader(http.StatusUnauthorized) // simulate expired token
			return
		}
		lastAuthHeader = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 1, "perPage": 500, "totalItems": 2, "totalPages": 1,
			"items": []map[string]any{{"id": "a"}, {"id": "b"}},
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	c := New(ts.URL)
	if _, err := c.AuthWithPassword(context.Background(), "leaf_nodes", "x", "y"); err != nil {
		t.Fatalf("AuthWithPassword: %v", err)
	}
	res, err := c.List(context.Background(), "things", 1, 500, "")
	if err != nil {
		t.Fatalf("List after re-auth: %v", err)
	}
	if len(res.Items) != 2 || res.TotalItems != 2 {
		t.Errorf("unexpected list result: %+v", res)
	}
	if authCount != 2 {
		t.Errorf("expected one re-auth (authCount=2), got %d", authCount)
	}
	if lastAuthHeader != "tok2" {
		t.Errorf("expected raw fresh token in Authorization header, got %q", lastAuthHeader)
	}
}

func TestListParsesPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("perPage"); got != "500" {
			t.Errorf("perPage not propagated, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 2, "perPage": 500, "totalItems": 7, "totalPages": 3,
			"items": []map[string]any{{"id": "x"}},
		})
	}))
	defer ts.Close()

	c := New(ts.URL)
	res, err := c.List(context.Background(), "things", 2, 500, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if res.Page != 2 || res.TotalPages != 3 || res.TotalItems != 7 {
		t.Errorf("pagination mis-parsed: %+v", res)
	}
}

func TestGetRawReturnsBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/leaf/operator-jwt" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"operator_jwt":"OPJWT"}`))
	}))
	defer ts.Close()

	c := New(ts.URL)
	b, err := c.GetRaw(context.Background(), "/api/leaf/operator-jwt")
	if err != nil {
		t.Fatalf("GetRaw: %v", err)
	}
	if string(b) != `{"operator_jwt":"OPJWT"}` {
		t.Errorf("unexpected raw body: %s", b)
	}
}
