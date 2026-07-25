package leafsync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"platform/internal/leafsync/pbclient"
)

func TestBuildLeafConf(t *testing.T) {
	conf := buildLeafConf(leafConfParams{
		OperatorJWT: "OPJWT",
		AccountPub:  "ACCOUNTPUB",
		AccountJWT:  "ACCJWT",
		Domain:      "edge-warehouse",
		HubLeafURL:  "nats-leaf://hub.example.com:7422",
		CredsName:   "edge.creds",
	})

	// Each directive a stock nats-server needs to come up as an offline,
	// operator-mode leaf bound to one preloaded account.
	wants := []string{
		`server_name: "edge-warehouse"`,
		`domain: "edge-warehouse"`, // JetStream domain isolates the edge from the hub
		`operator: "OPJWT"`,        // operator trust anchor
		`resolver: MEMORY`,         // no dynamic account resolver => no $SYS bridge
		`ACCOUNTPUB: "ACCJWT"`,     // resolver_preload: the single org account
		`url: "nats-leaf://hub.example.com:7422"`,
		`credentials: "edge.creds"`,
		`http: "127.0.0.1:8222"`, // localhost monitoring only
	}
	for _, w := range wants {
		if !strings.Contains(conf, w) {
			t.Errorf("nats-leaf.conf missing %q\n---\n%s", w, conf)
		}
	}
}

// fetchBootstrap must hit exactly one endpoint. `config` used to read nats_users
// and nats_accounts through the CRUD API; the leaf-node identity now has no grant
// on either, so a regression to a collection read would 404 in the field.
func TestFetchBootstrapUsesTheLeafRoute(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"domain":       "edge-warehouse",
			"code":         "warehouse",
			"creds":        "-----BEGIN NATS USER JWT-----",
			"account_jwt":  "ACCJWT",
			"account_pub":  "ACCOUNTPUB",
			"operator_jwt": "OPJWT"
		}`))
	}))
	defer srv.Close()

	bs, err := fetchBootstrap(context.Background(), pbclient.New(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 || paths[0] != "/api/leaf/bootstrap" {
		t.Errorf("requested %v, want exactly [/api/leaf/bootstrap]", paths)
	}
	if bs.Domain != "edge-warehouse" || bs.OperatorJWT != "OPJWT" || bs.AccountPub != "ACCOUNTPUB" {
		t.Errorf("decoded %+v", bs)
	}
}

// A half-provisioned leaf node must fail here with a named field, not produce a
// nats-leaf.conf containing empty directives.
func TestFetchBootstrapRejectsIncompleteResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// account_pub omitted: the account exists but has no public key yet.
		w.Write([]byte(`{
			"domain":       "edge-warehouse",
			"creds":        "CREDS",
			"account_jwt":  "ACCJWT",
			"operator_jwt": "OPJWT"
		}`))
	}))
	defer srv.Close()

	_, err := fetchBootstrap(context.Background(), pbclient.New(srv.URL))
	if err == nil {
		t.Fatal("expected an error for the missing field")
	}
	if !strings.Contains(err.Error(), "account_pub") {
		t.Errorf("error should name the missing field, got: %v", err)
	}
}
