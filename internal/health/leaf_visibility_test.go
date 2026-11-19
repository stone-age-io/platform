package health

// What a tenant can learn about its own leaf nodes, asked of a real hub with a
// real leaf attached.
//
// These tests pin nats-server behaviour, not ours. Site connectivity is a
// dashboard recipe (a widget aimed at $SYS.REQ.ACCOUNT.PING.CONNZ), with no
// platform route and no heartbeat, and the recipe works only while these hold.
// Fixtures cannot answer any of it, so it runs against a real hub and leaf.
//
//  1. CONNZ names each leaf by its `server_name`, a Thing's code, so a site is
//     identifiable, not merely countable. TestTenantCanSeeItsOwnLeafConnections.
//  2. The account, not any permission we write, blocks a tenant from the
//     operator-wide $SYS.REQ.SERVER.PING.* endpoints. That is why no tenant role
//     carries a $SYS publish deny. TestTenantCannotReachServerEndpoints.
//  3. Publish DENY beats publish ALLOW, so a `$SYS.>` deny kills account
//     monitoring whatever the allow list says, and fails as a bare timeout.
//     TestDenyingAllOfSysBlocksAccountMonitoring.
//  4. For the agent: $SRV discovery asked at the hub reaches a service behind a
//     leaf only if the leaf's uplink credential allows $SRV.>, which is why the
//     gateway role in internal/demoseed/contract.go carries it.
//     TestServiceDiscoveryCrossesALeafOnlyIfTheUplinkAllowsIt.
//  5. The same holds for a leaf's own JetStream asked by its domain
//     ($JS.<code>.API.>), which is why the gateway role carries $JS.*.API.>.
//     TestLeafJetStreamCrossesTheHubOnlyIfTheUplinkAllowsIt.

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
	"github.com/nats-io/nats.go/micro"
	"github.com/nats-io/nkeys"
)

// leafServerName is the leaf's `server_name`, which the agent's buildLeafConf
// sets to the JetStream domain, which is the Thing's code.
const leafServerName = "s01"

// The hub and the leaf both answer the account broadcast; replies are told
// apart by server name.
const hubServerName = "hub"

type leafWorld struct {
	opJWT  string
	sysPub string
	sysJWT string
	orgPub string
	orgJWT string
	orgKP  nkeys.KeyPair
}

// mintLeafWorld is mintOperatorWorld plus a tenant account.
func mintLeafWorld(t *testing.T) leafWorld {
	t.Helper()
	return mintLeafWorldWith(t, false)
}

// mintLeafWorldWith is mintLeafWorld with JetStream enabled on the tenant
// account when jetstream is true, as pb-nats does for an org with a storage
// plan.
func mintLeafWorldWith(t *testing.T, jetstream bool) leafWorld {
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
		// Explicit: a leaf connection limit of 0 would refuse the leaf.
		ac.Limits.LeafNodeConn = -1
		ac.Limits.Conn = -1
		if jetstream && name != "SYS" {
			ac.Limits.JetStreamLimits = jwt.JetStreamLimits{MemoryStorage: -1, DiskStorage: -1, Streams: -1, Consumer: -1}
		}
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

// userCreds mints a user in acc with a publish allow/deny, the shape pb-nats
// copies from nats_users.publish_permissions. Nil/nil is unrestricted.
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
// the tenant account, shaped like the agent's buildLeafConf output: operator, a
// MEMORY resolver preloaded with both the org and $SYS accounts, and a remote
// with an `account` key.
func startHubWithLeaf(t *testing.T, w leafWorld) *natsserver.Server {
	t.Helper()
	hub, _ := startHubWithLeafAs(t, w, userCreds(t, w.orgKP, "edge", nil, nil))
	return hub
}

// startHubWithLeafAs is startHubWithLeaf with the leaf's uplink credential
// chosen by the caller, returning the leaf as well as the hub.
func startHubWithLeafAs(t *testing.T, w leafWorld, uplinkCreds string) (hub, leaf *natsserver.Server) {
	t.Helper()
	return startHubWithLeafConf(t, w, uplinkCreds, "", "")
}

// startHubWithLeafConf is startHubWithLeafAs with extra top-level config for
// the hub and the leaf, such as a jetstream block.
func startHubWithLeafConf(t *testing.T, w leafWorld, uplinkCreds, hubExtra, leafExtra string) (hub, leaf *natsserver.Server) {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve leafnode port: %v", err)
	}
	leafPort := l.Addr().(*net.TCPAddr).Port
	l.Close()

	hub = startFromConf(t, "hub.conf", fmt.Sprintf(`
server_name: %q
operator: %q
system_account: %s
resolver: MEMORY
resolver_preload: {
  %s: %q
  %s: %q
}
leafnodes { port: %d }
%s
`, hubServerName, w.opJWT, w.sysPub, w.sysPub, w.sysJWT, w.orgPub, w.orgJWT, leafPort, hubExtra))

	remoteCreds := writeTemp(t, "edge.creds", uplinkCreds)
	leaf = startFromConf(t, "leaf.conf", fmt.Sprintf(`
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
%s
`, leafServerName, w.opJWT, w.orgPub, w.orgJWT, w.sysPub, w.sysJWT, leafPort, w.orgPub, remoteCreds, leafExtra))

	// Wait for the leaf to be named, not merely attached. NumLeafNodes() counts
	// the connection before `server_name` arrives, and CONNZ meanwhile shows a
	// nameless Leafnode. Waiting on the count flakes on a loaded CI runner.
	deadline := time.Now().Add(10 * time.Second)
	for !leafIsNamed(hub, leafServerName) {
		if time.Now().After(deadline) {
			t.Fatalf("no leaf named %q attached to the hub within 10s (NumLeafNodes=%d)",
				leafServerName, hub.NumLeafNodes())
		}
		time.Sleep(50 * time.Millisecond)
	}
	return hub, leaf
}

// leafIsNamed reports whether the hub holds a leaf connection carrying `name`.
//
// Leafz reports c.leaf.remoteServer and Connz reports c.opts.Name, but
// client.go fills c.opts before processLeafNodeConnect sets remoteServer, so a
// leaf named here is named in the hub's Connz. It says nothing about the leaf's
// own Connz (see TestTenantCanSeeItsOwnLeafConnections).
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
// credsPath. The bool is "answered": a permissions violation is reported
// asynchronously, so it surfaces here as a timeout.
//
// credsPath is passed in because t.TempDir() is named after the (sub)test, and
// a name Windows rejects (a `$`) would fail as a connection error.
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

// requestAll asks one monitoring subject and collects every reply.
// $SYS.REQ.ACCOUNT.PING.> is a broadcast that the hub and the leaf both answer,
// and nats.Request would take whichever arrives first.
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

	// Two servers answer this broadcast:
	//
	//	from "hub":  kind="Leafnode" name="s01"   <- a site, named
	//	from "s01":  kind="Leafnode" name=""      <- the leaf's own uplink
	//
	// The uplink is nameless because only the soliciting side sends CONNECT,
	// and Connz reports c.opts.Name. So a widget on this subject must aggregate
	// the replies and read the hub's, or it intermittently shows no site.
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

	// Without a name a tenant could only count its leaves, not say which is down.
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

// TestTenantCannotReachServerEndpoints is why no tenant role ships a $SYS
// publish deny. The server endpoints (LEAFZ answers for every account) live in
// the $SYS account, which a tenant account cannot reach. The credential below
// has no deny list, so the refusal is account scoping, not a rule of ours. A
// deny would add nothing and risk TestDenyingAllOfSysBlocksAccountMonitoring.
// The roles in internal/demoseed/contract.go rely on this.
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

// uplinkCreds mints a leaf uplink user whose SUBSCRIBE list is subAllow (nil is
// unrestricted) and whose publish is unrestricted. userCreds varies publish.
func uplinkCreds(t *testing.T, acc nkeys.KeyPair, subAllow []string) string {
	t.Helper()
	ukp, _ := nkeys.CreateUser()
	upub, _ := ukp.PublicKey()
	uc := jwt.NewUserClaims(upub)
	uc.Name = "edge"
	uc.Permissions.Sub.Allow = subAllow
	userJWT, err := uc.Encode(acc)
	if err != nil {
		t.Fatalf("encode uplink user: %v", err)
	}
	seed, _ := ukp.Seed()
	raw, err := jwt.FormatUserConfig(userJWT, seed)
	if err != nil {
		t.Fatalf("FormatUserConfig uplink: %v", err)
	}
	return string(raw)
}

// connectTo opens a connection to srv as the holder of credsPath, closed when
// the test ends.
func connectTo(t *testing.T, srv *natsserver.Server, credsPath string) *nats.Conn {
	t.Helper()
	nc, err := nats.Connect(srv.ClientURL(),
		nats.UserCredentials(credsPath),
		nats.MaxReconnects(0),
		nats.NoReconnect(),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			t.Logf("async: %v", err)
		}),
	)
	if err != nil {
		t.Fatalf("connect to %s: %v", srv.Name(), err)
	}
	t.Cleanup(nc.Close)
	return nc
}

// TestServiceDiscoveryCrossesALeafOnlyIfTheUplinkAllowsIt: a micro service on a
// leaf (the agent) answers $SRV discovery asked at the hub only when the leaf's
// uplink credential lets $SRV.> through. The agent's own grant is not enough.
//
// Each case first gets an answer to an ordinary command, because a silent
// $SRV.PING also looks like a leaf that never attached. The service subscribes
// to $SRV before the command endpoint, and the leaf propagates interest in
// order, so once the command answers any allowed $SRV interest is at the hub.
func TestServiceDiscoveryCrossesALeafOnlyIfTheUplinkAllowsIt(t *testing.T) {
	w := mintLeafWorld(t)

	// No `$` in these names: TempDir uses them, and Windows rejects it.
	cases := []struct {
		name      string
		uplinkSub []string
		wantPing  bool
	}{
		{name: "uplink allows discovery", uplinkSub: nil, wantPing: true},
		{name: "uplink without discovery", uplinkSub: []string{"agents.>", "_INBOX.>"}, wantPing: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hub, leaf := startHubWithLeafAs(t, w, uplinkCreds(t, w.orgKP, tc.uplinkSub))

			// The agent is unrestricted, so only the uplink can stop discovery.
			agent := connectTo(t, leaf, writeTemp(t, "agent.creds", userCreds(t, w.orgKP, "agent", nil, nil)))
			svc, err := micro.AddService(agent, micro.Config{Name: "stone-agent", Version: "1.0.0"})
			if err != nil {
				t.Fatalf("AddService: %v", err)
			}
			pong := micro.HandlerFunc(func(r micro.Request) { _ = r.Respond([]byte(`{"status":"pong"}`)) })
			if err := svc.AddEndpoint("ping", pong, micro.WithEndpointSubject("agents."+leafServerName+".cmd.ping")); err != nil {
				t.Fatalf("AddEndpoint: %v", err)
			}
			if err := agent.Flush(); err != nil {
				t.Fatalf("flush: %v", err)
			}

			// A tenant session at the hub, where the console connects.
			console := connectTo(t, hub, writeTemp(t, "console.creds", userCreds(t, w.orgKP, "console", nil, nil)))

			deadline := time.Now().Add(10 * time.Second)
			for {
				if _, err := console.Request("agents."+leafServerName+".cmd.ping", nil, 500*time.Millisecond); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("a command to the service on the leaf never answered from the hub, so the discovery result below would mean nothing")
				}
				time.Sleep(50 * time.Millisecond)
			}

			_, err = console.Request("$SRV.PING.stone-agent", nil, time.Second)
			if answered := err == nil; answered != tc.wantPing {
				t.Errorf("$SRV.PING from the hub answered = %v, want %v (uplink subscribe allow: %q)",
					answered, tc.wantPing, tc.uplinkSub)
			}
		})
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
		// The shape people revert to.
		{name: "blanket sys deny", deny: []string{"$SYS.>"}, want: false},

		// Does not work: deny beats allow, and the symptom is a bare timeout.
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

	// Parent t: these subtest names are not safe as TempDir path segments.
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

// TestLeafJetStreamCrossesTheHubOnlyIfTheUplinkAllowsIt: a tenant session at
// the hub reaches a leaf's own JetStream, through its domain
// ($JS.<code>.API.>), only when the leaf's uplink credential may subscribe to
// it. That is why the gateway role in internal/demoseed/contract.go carries
// $JS.*.API.>, and what `stone js pull --domain <code>` depends on. Without
// it the hub has no interest to forward along, so the caller gets "no
// responders" at once rather than a timeout.
func TestLeafJetStreamCrossesTheHubOnlyIfTheUplinkAllowsIt(t *testing.T) {
	w := mintLeafWorldWith(t, true)

	// The seeded gateway's subscribe list before $JS.*.API.> was added.
	gateway := []string{"telemetry.>", "event.>", "status.>", "gateway.>", "acc.>", "kiosk.>", "cmd.>", "config.>", "_INBOX.>", "$SRV.>"}
	cases := []struct {
		name      string
		uplinkSub []string
		want      bool
	}{
		{name: "gateway without the JetStream API", uplinkSub: gateway, want: false},
		{name: "gateway with the JetStream API", uplinkSub: append(slices.Clone(gateway), "$JS.*.API.>"), want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			js := func(domain string) string {
				return fmt.Sprintf("jetstream { domain: %q, store_dir: %q }", domain, filepath.ToSlash(t.TempDir()))
			}
			hub, leaf := startHubWithLeafConf(t, w, uplinkCreds(t, w.orgKP, tc.uplinkSub), js(hubServerName), js(leafServerName))

			// The leaf's JetStream is up, asked on the leaf itself, and a
			// responder on the leaf answers the hub on a subject the uplink
			// allows in both cases. So a silence below is the uplink, not a slow
			// start or a leaf that never attached.
			local := connectTo(t, leaf, writeTemp(t, "local.creds", userCreds(t, w.orgKP, "local", nil, nil)))
			if got := jsDomain(t, local, "$JS.API.INFO", 10*time.Second); got != leafServerName {
				t.Fatalf("the leaf's own JetStream answered with domain %q, want %q", got, leafServerName)
			}
			if _, err := local.Subscribe("gateway."+leafServerName+".ping", func(m *nats.Msg) { _ = m.Respond([]byte("pong")) }); err != nil {
				t.Fatalf("subscribe: %v", err)
			}
			if err := local.Flush(); err != nil {
				t.Fatalf("flush: %v", err)
			}
			console := connectTo(t, hub, writeTemp(t, "console.creds", userCreds(t, w.orgKP, "console", nil, nil)))
			deadline := time.Now().Add(10 * time.Second)
			for {
				if _, err := console.Request("gateway."+leafServerName+".ping", nil, 500*time.Millisecond); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("a request to the leaf never answered from the hub, so the JetStream result below would mean nothing")
				}
				time.Sleep(50 * time.Millisecond)
			}

			msg, err := console.Request("$JS."+leafServerName+".API.INFO", nil, 2*time.Second)
			if answered := err == nil; answered != tc.want {
				t.Fatalf("$JS.%s.API.INFO from the hub answered = %v (err %v), want %v (uplink subscribe allow: %q)",
					leafServerName, answered, err, tc.want, tc.uplinkSub)
			}
			if err == nil {
				var info struct {
					Domain string `json:"domain"`
				}
				if err := json.Unmarshal(msg.Data, &info); err != nil || info.Domain != leafServerName {
					t.Fatalf("answered by domain %q (%v), want the leaf's %q: %s", info.Domain, err, leafServerName, msg.Data)
				}
			}
		})
	}
}

// jsDomain asks subj (a JetStream account INFO) until it answers, and returns
// the domain in the reply.
func jsDomain(t *testing.T, nc *nats.Conn, subj string, within time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		msg, err := nc.Request(subj, nil, 500*time.Millisecond)
		if err == nil {
			var info struct {
				Domain string `json:"domain"`
			}
			if err := json.Unmarshal(msg.Data, &info); err != nil {
				t.Fatalf("decode %s reply: %v: %s", subj, err, msg.Data)
			}
			return info.Domain
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never answered: %v", subj, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
