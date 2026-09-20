package health

// What a tenant can learn about its own leaf nodes, asked of a real hub with a
// real leaf attached.
//
// These tests assert nats-server's behaviour, not ours. Nothing in the platform
// ships a view built on them: a console badge for site connectivity was built
// and removed, because nothing in the schema marks which Things are gateways, so
// it rendered on every device and polled a whole account's connection list every
// 15 seconds to tell temperature probes they had no leaf node. The capability is
// a dashboard recipe now -- a Publisher or Button widget aimed at
// $SYS.REQ.ACCOUNT.PING.CONNZ -- which is why these still matter: the recipe is
// only usable if the three facts below hold, and none of them is ours to
// guarantee.
//
// There is deliberately no platform route and no `leaf_status` heartbeat for
// this either. A heartbeat travels over the very link that breaks, so its
// absence cannot distinguish "edge down" from "WAN down" from "agent crashed",
// and the Control Plane could not read one anyway -- it holds the operator and
// $SYS and no credential inside any tenant account. The hub always knows.
//
// Three facts, and the third is the one that silently dismantles the other two:
//
//  1. $SYS.REQ.ACCOUNT.PING.CONNZ lists leaf connections, and names each one by
//     the leaf server's `server_name` -- which is a Thing's code. A site is
//     therefore identifiable, not merely countable (STATZ gives a bare count).
//  2. The account scope is enforced by the SERVER, not by any permission we
//     write. A tenant cannot reach $SYS.REQ.SERVER.PING.LEAFZ -- every account's
//     leaves -- with a credential carrying no deny list at all, because those
//     endpoints are served inside the $SYS ACCOUNT and an account is a closed
//     subject namespace. That is TestTenantCannotReachServerEndpoints, and it is
//     why no tenant role here ships a $SYS publish deny: a deny would restate
//     this boundary somewhere weaker and strictly worse.
//  3. Publish DENY beats publish ALLOW. A user carrying `$SYS.>` in its deny
//     list cannot reach the account endpoints no matter what its allow list
//     says -- and the failure surfaces as a request timeout, with the real
//     reason arriving asynchronously on the connection's error handler. That is
//     TestDenyingAllOfSysBlocksAccountMonitoring, which exists so nobody
//     "re-tightens" tenant permissions back to a blanket $SYS deny and then
//     debugs a dead widget for a day.
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
	"slices"
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

// The hub answers the account broadcast alongside the leaf, so replies have to
// be told apart by the server that sent them. A constant rather than the same
// literal in two places.
const hubServerName = "hub"

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
server_name: %q
operator: %q
system_account: %s
resolver: MEMORY
resolver_preload: {
  %s: %q
  %s: %q
}
leafnodes { port: %d }
`, hubServerName, w.opJWT, w.sysPub, w.sysPub, w.sysJWT, w.orgPub, w.orgJWT, leafPort))

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

	// Wait for the leaf to be attached AND NAMED, not merely attached.
	//
	// NumLeafNodes() counts the connection, which the hub registers as soon as
	// the TCP session is accepted. `server_name` arrives later, in the leaf's
	// own INFO -- so there is a window where the count is 1 and CONNZ reports a
	// Leafnode entry with an empty name. That window is what every assertion
	// here actually depends on closing: a site is identified BY its name, and a
	// nameless leaf is indistinguishable from a leaf for another Thing.
	//
	// Waiting on the count was therefore waiting on a proxy for the condition,
	// and it failed exactly as you would expect a proxy to fail: green on a
	// developer machine for months, then `name = ""` on a loaded CI runner. Wait
	// for the fact the test needs.
	deadline := time.Now().Add(10 * time.Second)
	for !leafIsNamed(hub, leafServerName) {
		if time.Now().After(deadline) {
			t.Fatalf("no leaf named %q attached to the hub within 10s (NumLeafNodes=%d)",
				leafServerName, hub.NumLeafNodes())
		}
		time.Sleep(50 * time.Millisecond)
	}
	return hub
}

// leafIsNamed reports whether the hub holds a leaf connection carrying `name`.
//
// Leafz reports c.leaf.remoteServer where Connz reports c.opts.Name. Different
// fields, but the ordering makes this a sound wait: client.go fills c.opts from
// the CONNECT proto before dispatching to processLeafNodeConnect, which is what
// sets remoteServer -- so a leaf named here is already named in the hub's Connz.
//
// What it does NOT establish is anything about the LEAF's own Connz, which also
// answers the account broadcast and reports its uplink to the hub unnamed. See
// TestTenantCanSeeItsOwnLeafConnections.
func leafIsNamed(hub *natsserver.Server, name string) bool {
	lz, err := hub.Leafz(&natsserver.LeafzOptions{})
	if err != nil || lz == nil {
		return false
	}
	for _, leaf := range lz.Leafs {
		if leaf.Name == name {
			return true
		}
	}
	return false
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

// requestAll asks one monitoring subject and collects EVERY reply, not the first.
//
// $SYS.REQ.ACCOUNT.PING.> is a broadcast: every server serving the account
// answers it, and with a leaf attached that is two -- the hub and the leaf
// itself. nats.Request takes whichever arrives first, which is a coin toss under
// load and is not what "what is connected to my account" means.
func requestAll(t *testing.T, hub *natsserver.Server, credsPath, subject string, window time.Duration) [][]byte {
	t.Helper()
	nc, err := nats.Connect(hub.ClientURL(),
		nats.UserCredentials(credsPath),
		nats.Timeout(3*time.Second),
		nats.MaxReconnects(0),
		nats.NoReconnect(),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			t.Logf("async: %v", err)
		}),
	)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	sub, err := nc.SubscribeSync(nats.NewInbox())
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := nc.PublishRequest(subject, sub.Subject, nil); err != nil {
		t.Fatalf("publish %s: %v", subject, err)
	}
	if err := nc.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	var replies [][]byte
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		msg, err := sub.NextMsg(time.Until(deadline))
		if err != nil {
			break
		}
		replies = append(replies, msg.Data)
	}
	return replies
}

func TestTenantCanSeeItsOwnLeafConnections(t *testing.T) {
	w := mintLeafWorld(t)
	hub := startHubWithLeaf(t, w)
	tenant := writeTemp(t, "tenant.creds",
		userCreds(t, w.orgKP, "acme-user", []string{"$SYS.REQ.ACCOUNT.PING.>", "_INBOX.>"}, nil))

	// EVERY reply, not the first -- and that distinction is the widget recipe's
	// problem as much as this test's.
	//
	// $SYS.REQ.ACCOUNT.PING.CONNZ is a broadcast, and with a leaf attached TWO
	// servers answer it:
	//
	//	from "hub":  kind="Leafnode" name="s01"   <- a site, named
	//	from "s01":  kind="Leafnode" name=""      <- the leaf's own uplink
	//
	// The leaf's row is its connection back to the hub. It is nameless because
	// only the SOLICITING side sends CONNECT, so on the leaf that connection has
	// no c.opts.Name -- and c.opts.Name is precisely what Connz reports
	// (monitor.go). Nothing is wrong with the row; it is simply not a site.
	//
	// This test used to take the first reply and then require every Leafnode row
	// in it to be named. That holds whenever the hub answers first, which is
	// almost always, on an idle machine, for months. When the leaf won the race
	// the only row present was its unnamed uplink, and CI failed with
	// name = "" -- twice now, both times on a commit touching nothing near this
	// package. The earlier fix, making startHubWithLeaf wait for the leaf to be
	// NAMED, was a real repair to a different defect and could never have fixed
	// this one.
	//
	// So a Publisher widget aimed at this subject must aggregate the replies and
	// read the HUB's, or it will intermittently render a site list holding one
	// nameless entry and no site.
	replies := requestAll(t, hub, tenant, "$SYS.REQ.ACCOUNT.PING.CONNZ", 1500*time.Millisecond)
	if len(replies) == 0 {
		t.Fatal("a tenant could not read CONNZ for its own account; the dashboard recipe for site connectivity does not work and would need a platform route after all")
	}

	type connzReply struct {
		Server struct {
			Name string `json:"name"`
		} `json:"server"`
		Data struct {
			Connections []struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			} `json:"connections"`
		} `json:"data"`
	}

	var hubReply *connzReply
	for _, body := range replies {
		var r connzReply
		if err := json.Unmarshal(body, &r); err != nil {
			t.Fatalf("decode CONNZ: %v", err)
		}
		if r.Server.Name == hubServerName {
			hubReply = &r
		}
	}
	if hubReply == nil {
		t.Fatalf("no CONNZ reply from %q among %d replies; the hub is the only server that can name a site", hubServerName, len(replies))
	}

	// The whole identity story. Without a name here a tenant could only count its
	// leaves, never say WHICH site is offline.
	var leafNames []string
	for _, c := range hubReply.Data.Connections {
		if c.Kind == "Leafnode" {
			leafNames = append(leafNames, c.Name)
		}
	}
	if len(leafNames) == 0 {
		t.Fatal("the hub answered CONNZ but listed no Leafnode connection")
	}
	if !slices.Contains(leafNames, leafServerName) {
		t.Errorf("hub CONNZ named no leaf %q; leaf names seen: %q", leafServerName, leafNames)
	}
}

// TestTenantCannotReachServerEndpoints is why no tenant role in this platform
// ships a $SYS publish deny.
//
// $SYS.REQ.SERVER.PING.LEAFZ answers for EVERY account's leaves, so a tenant
// must never reach it -- and it never can, because those endpoints are served
// inside the $SYS ACCOUNT and an account is a closed subject namespace. The
// credential below carries an allow list and NO deny list whatsoever, which is
// the point: the refusal is the server's account scoping, not a subject rule of
// ours.
//
// A `$SYS.>` or `$SYS.REQ.SERVER.>` deny on a tenant role therefore adds no
// protection, while risking the failure in
// TestDenyingAllOfSysBlocksAccountMonitoring. Both roles in
// internal/demoseed/contract.go used to carry one; this is the assertion that
// licensed removing them.
func TestTenantCannotReachServerEndpoints(t *testing.T) {
	w := mintLeafWorld(t)
	hub := startHubWithLeaf(t, w)
	tenant := writeTemp(t, "tenant.creds",
		userCreds(t, w.orgKP, "acme-user", []string{"$SYS.REQ.ACCOUNT.PING.>", "_INBOX.>"}, nil))

	for _, subject := range []string{
		"$SYS.REQ.SERVER.PING.LEAFZ",
		"$SYS.REQ.SERVER.PING.CONNZ",
		"$SYS.REQ.SERVER.PING.VARZ",
	} {
		if _, ok := request(t, hub, tenant, subject); ok {
			t.Errorf("a tenant reached %s with no deny list; account scoping is not being enforced, "+
				"and the roles in internal/demoseed/contract.go need their $SYS deny back", subject)
		}
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
