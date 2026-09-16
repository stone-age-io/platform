package health

// What a tenant can learn about its own leaf nodes, asked of a real hub with a
// real leaf attached.
//
// These tests assert nats-server's behaviour, not ours. They are here because
// the console's site-connectivity view is built directly on them: the browser
// connects as an ordinary org-account user and asks NATS whether that
// organization's leaf nodes are attached. There is deliberately no platform
// route and no `leaf_status` heartbeat behind that view -- a heartbeat travels
// over the very link that breaks, so its absence cannot distinguish "edge down"
// from "WAN down" from "agent crashed". The hub always knows.
//
// Two facts hold the design up, and a third would silently dismantle it:
//
//  1. $SYS.REQ.ACCOUNT.PING.CONNZ lists leaf connections, and names each one by
//     the leaf server's `server_name` -- which is a Thing's code. A site is
//     therefore identifiable, not merely countable (STATZ gives a bare count).
//  2. The account scope is enforced by the server. A tenant cannot reach
//     $SYS.REQ.SERVER.PING.LEAFZ, which would be every account's leaves.
//  3. Publish DENY beats publish ALLOW. A user carrying `$SYS.>` in its deny
//     list cannot reach the account endpoints no matter what its allow list
//     says -- and the failure surfaces as a request timeout, with the real
//     reason arriving asynchronously on the connection's error handler. That is
//     TestDenyingAllOfSysBlocksAccountMonitoring, which exists so nobody
//     "re-tightens" tenant permissions back to a blanket $SYS deny and then
//     debugs the console for a day.
//
// Fixtures cannot answer any of this, which is the same reason
// TestBuildLeafConfIsAcceptedByNATSServer runs the generated config through the
// real server rather than grepping it.

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

// leafServerName is the leaf's `server_name`, which buildLeafConf sets to the
// JetStream domain, which this design sets to the Thing's code.
const leafServerName = "s01"

type leafWorld struct {
	opJWT  string
	sysPub string
	sysJWT string
	orgPub string
	orgJWT string
	orgKP  nkeys.KeyPair
}

// mintLeafWorld is mintOperatorWorld plus a tenant account, so a question can be
// asked from inside an organization rather than only from $SYS.
func mintLeafWorld(t *testing.T) leafWorld {
	t.Helper()

	okp, err := nkeys.CreateOperator()
	if err != nil {
		t.Fatalf("create operator key: %v", err)
	}
	opub, _ := okp.PublicKey()

	mkAccount := func(name string) (nkeys.KeyPair, string, string) {
		kp, _ := nkeys.CreateAccount()
		pub, _ := kp.PublicKey()
		ac := jwt.NewAccountClaims(pub)
		ac.Name = name
		// Explicit rather than trusting the zero value: a leaf connection limit
		// of 0 would refuse the very thing under test.
		ac.Limits.LeafNodeConn = -1
		ac.Limits.Conn = -1
		encoded, err := ac.Encode(okp)
		if err != nil {
			t.Fatalf("encode account %s: %v", name, err)
		}
		return kp, pub, encoded
	}

	sysKP, sysPub, sysJWT := mkAccount("SYS")
	orgKP, orgPub, orgJWT := mkAccount("acme")
	_ = sysKP

	oc := jwt.NewOperatorClaims(opub)
	oc.Name = "stone-age.io"
	oc.SystemAccount = sysPub
	opJWT, err := oc.Encode(okp)
	if err != nil {
		t.Fatalf("encode operator: %v", err)
	}

	return leafWorld{
		opJWT:  opJWT,
		sysPub: sysPub,
		sysJWT: sysJWT,
		orgPub: orgPub,
		orgJWT: orgJWT,
		orgKP:  orgKP,
	}
}

// userCreds mints a user in acc with an explicit publish allow/deny, matching
// the shape pb-nats copies verbatim from nats_users.publish_permissions into the
// signed JWT. Nil/nil is an unrestricted user.
func userCreds(t *testing.T, acc nkeys.KeyPair, name string, allow, deny []string) string {
	t.Helper()
	ukp, _ := nkeys.CreateUser()
	upub, _ := ukp.PublicKey()
	uc := jwt.NewUserClaims(upub)
	uc.Name = name
	uc.Permissions.Pub.Allow = allow
	uc.Permissions.Pub.Deny = deny
	userJWT, err := uc.Encode(acc)
	if err != nil {
		t.Fatalf("encode user %s: %v", name, err)
	}
	seed, _ := ukp.Seed()
	raw, err := jwt.FormatUserConfig(userJWT, seed)
	if err != nil {
		t.Fatalf("FormatUserConfig %s: %v", name, err)
	}
	return string(raw)
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func startFromConf(t *testing.T, name, conf string) *natsserver.Server {
	t.Helper()
	opts, err := natsserver.ProcessConfigFile(writeTemp(t, name, conf))
	if err != nil {
		t.Fatalf("nats-server rejected %s: %v", name, err)
	}
	return startTestNATS(t, opts)
}

// startHubWithLeaf brings up an operator-mode hub and attaches one leaf bound to
// the tenant account, shaped like the file buildLeafConf writes: operator, a
// MEMORY resolver preloaded with BOTH the org and $SYS accounts, and a remote
// carrying an `account` key.
func startHubWithLeaf(t *testing.T, w leafWorld) *natsserver.Server {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve leafnode port: %v", err)
	}
	leafPort := l.Addr().(*net.TCPAddr).Port
	l.Close()

	hub := startFromConf(t, "hub.conf", fmt.Sprintf(`
server_name: "hub"
operator: %q
system_account: %s
resolver: MEMORY
resolver_preload: {
  %s: %q
  %s: %q
}
leafnodes { port: %d }
`, w.opJWT, w.sysPub, w.sysPub, w.sysJWT, w.orgPub, w.orgJWT, leafPort))

	remoteCreds := writeTemp(t, "edge.creds", userCreds(t, w.orgKP, "edge", nil, nil))
	startFromConf(t, "leaf.conf", fmt.Sprintf(`
server_name: %q
operator: %q
resolver: MEMORY
resolver_preload: {
  %s: %q
  %s: %q
}
leafnodes {
  remotes: [
    { url: "nats-leaf://127.0.0.1:%d", account: %s, credentials: %q }
  ]
}
`, leafServerName, w.opJWT, w.orgPub, w.orgJWT, w.sysPub, w.sysJWT, leafPort, w.orgPub, remoteCreds))

	deadline := time.Now().Add(10 * time.Second)
	for hub.NumLeafNodes() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("leaf node never attached to the hub")
		}
		time.Sleep(50 * time.Millisecond)
	}
	return hub
}

// request asks one monitoring subject as the holder of the creds file at
// credsPath. The bool is "answered", not "succeeded": a permissions violation
// surfaces here as a timeout, because the server reports the violation
// asynchronously on the error handler.
//
// credsPath is passed in rather than written here on purpose. t.TempDir() names
// the directory after the test, and a subtest name containing characters
// Windows rejects (a `$`, say) yields a path that cannot be created -- which
// presents as a connection failure in a test about permissions.
func request(t *testing.T, hub *natsserver.Server, credsPath, subject string) ([]byte, bool) {
	t.Helper()
	nc, err := nats.Connect(hub.ClientURL(),
		nats.UserCredentials(credsPath),
		nats.Timeout(3*time.Second),
		nats.MaxReconnects(0),
		nats.NoReconnect(),
		// The permissions violation lands here, not on the Request call.
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			t.Logf("async: %v", err)
		}),
	)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	msg, err := nc.Request(subject, nil, 2*time.Second)
	if err != nil {
		return nil, false
	}
	return msg.Data, true
}

func TestTenantCanSeeItsOwnLeafConnections(t *testing.T) {
	w := mintLeafWorld(t)
	hub := startHubWithLeaf(t, w)
	tenant := writeTemp(t, "tenant.creds",
		userCreds(t, w.orgKP, "acme-user", []string{"$SYS.REQ.ACCOUNT.PING.>", "_INBOX.>"}, nil))

	body, ok := request(t, hub, tenant, "$SYS.REQ.ACCOUNT.PING.CONNZ")
	if !ok {
		t.Fatal("a tenant could not read CONNZ for its own account; the console would need a platform route after all")
	}

	var connz struct {
		Data struct {
			Connections []struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			} `json:"connections"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &connz); err != nil {
		t.Fatalf("decode CONNZ: %v", err)
	}

	var found bool
	for _, c := range connz.Data.Connections {
		if c.Kind != "Leafnode" {
			continue
		}
		found = true
		// The whole identity story. Without a name here a tenant could only
		// count its leaves, never say WHICH site is offline.
		if c.Name != leafServerName {
			t.Errorf("leaf connection name = %q, want the leaf server_name %q", c.Name, leafServerName)
		}
	}
	if !found {
		t.Error("CONNZ answered but listed no Leafnode connection")
	}

	// The scope is the server's to enforce, not ours. SERVER.PING.LEAFZ would be
	// every account's leaves.
	if _, ok := request(t, hub, tenant, "$SYS.REQ.SERVER.PING.LEAFZ"); ok {
		t.Error("a tenant reached SERVER.PING.LEAFZ; account scoping is not being enforced")
	}
}

func TestDenyingAllOfSysBlocksAccountMonitoring(t *testing.T) {
	w := mintLeafWorld(t)
	hub := startHubWithLeaf(t, w)

	cases := []struct {
		name  string
		allow []string
		deny  []string
		want  bool
	}{
		// The shape tenant roles carried before this feature. Kept as a case
		// rather than a comment because it is the one people revert to.
		{name: "blanket sys deny", deny: []string{"$SYS.>"}, want: false},

		// The obvious fix, which does not work: deny beats allow in NATS, so
		// adding the allow changes nothing. Anyone who tries this sees a
		// request timeout and no hint that permissions are involved.
		{
			name:  "blanket deny plus a narrower allow",
			allow: []string{"$SYS.REQ.ACCOUNT.PING.>"},
			deny:  []string{"$SYS.>"},
			want:  false,
		},

		// Two shapes that do work.
		{name: "deny only the server endpoints", deny: []string{"$SYS.REQ.SERVER.>"}, want: true},
		{
			name:  "allow-list the account endpoints",
			allow: []string{"$SYS.REQ.ACCOUNT.PING.>", "_INBOX.>"},
			want:  true,
		},
	}

	// Written with the parent t: a subtest's TempDir is named after the subtest,
	// and these names are not all safe as path segments on every OS.
	paths := make([]string, len(cases))
	for i, tc := range cases {
		paths[i] = writeTemp(t, fmt.Sprintf("case%d.creds", i),
			userCreds(t, w.orgKP, "scoped", tc.allow, tc.deny))
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := request(t, hub, paths[i], "$SYS.REQ.ACCOUNT.PING.CONNZ")
			if ok != tc.want {
				t.Errorf("answered = %v, want %v", ok, tc.want)
			}
		})
	}
}
