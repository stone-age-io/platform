package leafsync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	njwt "github.com/nats-io/jwt/v2"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nkeys"

	"platform/internal/leafsync/pbclient"
)

func TestBuildLeafConf(t *testing.T) {
	conf := buildLeafConf(leafConfParams{
		OperatorJWT:   "OPJWT",
		AccountPub:    "ACCOUNTPUB",
		AccountJWT:    "ACCJWT",
		SysAccountPub: "SYSPUB",
		SysAccountJWT: "SYSJWT",
		Domain:        "edge-warehouse",
		HubLeafURL:    "nats-leaf://hub.example.com:7422",
		CredsName:     "edge.creds",
	})

	// Each directive a stock nats-server needs to come up as an offline,
	// operator-mode leaf bound to one preloaded account.
	wants := []string{
		`server_name: "edge-warehouse"`,
		`domain: "edge-warehouse"`, // JetStream domain isolates the edge from the hub
		`operator: "OPJWT"`,        // operator trust anchor
		`resolver: MEMORY`,         // no dynamic account resolver
		`ACCOUNTPUB: "ACCJWT"`,     // resolver_preload: the org account
		`SYSPUB: "SYSJWT"`,         // ...and the $SYS account the operator JWT names
		`url: "nats-leaf://hub.example.com:7422"`,
		`account: "ACCOUNTPUB"`, // required on every remote in operator mode
		`credentials: "edge.creds"`,
		`http: "127.0.0.1:8222"`, // localhost monitoring only
	}
	for _, w := range wants {
		if !strings.Contains(conf, w) {
			t.Errorf("nats-leaf.conf missing %q\n---\n%s", w, conf)
		}
	}
}

// The string assertions above cannot tell whether nats-server will actually
// accept the result, and for a long time it would not have: the config was
// missing the remote's `account` key and the $SYS account preload, so every
// generated nats-leaf.conf was rejected before the server started. Both bugs are
// invisible to a Contains() check and neither needs a port or a network — the
// server validates the whole config while constructing itself.
//
// So: build a real operator/$SYS/account/user set, run the real generator, and
// hand the output to the real nats-server config loader.
func TestBuildLeafConfIsAcceptedByNATSServer(t *testing.T) {
	okp, err := nkeys.CreateOperator()
	if err != nil {
		t.Fatalf("create operator key: %v", err)
	}
	opub, _ := okp.PublicKey()

	// $SYS, as pb-nats seeds it centrally.
	sysKP, _ := nkeys.CreateAccount()
	sysPub, _ := sysKP.PublicKey()
	sysJWT, err := njwt.NewAccountClaims(sysPub).Encode(okp)
	if err != nil {
		t.Fatalf("encode system account: %v", err)
	}

	// pb-nats records the system account inside the operator JWT
	// (internal/jwt/generator.go). That is what forces the preload below.
	oc := njwt.NewOperatorClaims(opub)
	oc.Name = "stone-age.io"
	oc.SystemAccount = sysPub
	opJWT, err := oc.Encode(okp)
	if err != nil {
		t.Fatalf("encode operator: %v", err)
	}

	akp, _ := nkeys.CreateAccount()
	apub, _ := akp.PublicKey()
	ac := njwt.NewAccountClaims(apub)
	ac.Name = "acme"
	ac.Limits.JetStreamLimits.DiskStorage = -1
	ac.Limits.JetStreamLimits.MemoryStorage = -1
	accJWT, err := ac.Encode(okp)
	if err != nil {
		t.Fatalf("encode account: %v", err)
	}

	conf := buildLeafConf(leafConfParams{
		OperatorJWT:   opJWT,
		AccountPub:    apub,
		AccountJWT:    accJWT,
		SysAccountPub: sysPub,
		SysAccountJWT: sysJWT,
		Domain:        "edge-warehouse",
		HubLeafURL:    "nats-leaf://hub.example.com:7422",
		CredsName:     "edge.creds",
	})

	path := filepath.Join(t.TempDir(), LeafConfName)
	if err := os.WriteFile(path, []byte(conf), 0o644); err != nil {
		t.Fatalf("write conf: %v", err)
	}

	opts, err := natsserver.ProcessConfigFile(path)
	if err != nil {
		t.Fatalf("nats-server rejected the generated config: %v\n---\n%s", err, conf)
	}
	// NewServer is where operator-mode validation runs (remote account keys,
	// system account resolution). It binds no ports, so this stays hermetic.
	srv, err := natsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("nats-server could not build a server from the generated config: %v\n---\n%s", err, conf)
	}
	srv.Shutdown()
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
			"domain":          "edge-warehouse",
			"code":            "warehouse",
			"creds":           "-----BEGIN NATS USER JWT-----",
			"account_jwt":     "ACCJWT",
			"account_pub":     "ACCOUNTPUB",
			"operator_jwt":    "OPJWT",
			"sys_account_jwt": "SYSJWT",
			"sys_account_pub": "SYSPUB"
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
