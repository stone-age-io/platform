package health

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nkeys"
)

func TestHostPort(t *testing.T) {
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{in: "nats://localhost:4222", want: "localhost:4222"},
		// An omitted port is the client default, not an error.
		{in: "nats://localhost", want: "localhost:4222"},
		// Without a scheme, url.Parse would read this as a path.
		{in: "localhost:4222", want: "localhost:4222"},
		{in: "tls://nats.example.com:4223", want: "nats.example.com:4223"},
		{in: "nats://[::1]:4222", want: "[::1]:4222"},
		{in: "", wantErr: true},
		{in: "://nope", wantErr: true},
	}

	for _, tc := range cases {
		got, err := hostPort(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("hostPort(%q) = %q, want an error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("hostPort(%q) errored: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("hostPort(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// startTestNATS runs a real nats-server on a random port. A canned INFO string
// would assert what we believe the protocol is, not what it is.
func startTestNATS(t *testing.T, opts *natsserver.Options) *natsserver.Server {
	t.Helper()
	if opts == nil {
		opts = &natsserver.Options{}
	}
	opts.Port = -1 // random free port
	opts.NoLog = true
	opts.NoSigs = true

	ns, err := natsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("could not create test NATS server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		ns.Shutdown()
		t.Fatal("test NATS server never became ready")
	}
	t.Cleanup(func() {
		ns.Shutdown()
		ns.WaitForShutdown()
	})
	return ns
}

func TestDialInfoReadsARealServersGreeting(t *testing.T) {
	ns := startTestNATS(t, &natsserver.Options{ServerName: "readiness-test"})

	info, err := DialInfo(context.Background(), ns.ClientURL(), 3*time.Second)
	if err != nil {
		t.Fatalf("DialInfo against a live server failed: %v", err)
	}
	if info.ServerName != "readiness-test" {
		t.Errorf("ServerName = %q, want %q", info.ServerName, "readiness-test")
	}
	if info.Version == "" {
		t.Error("Version should be populated from the INFO greeting")
	}
	// JetStream is off here, and the check warns on that, so the field must
	// reflect the server.
	if info.JetStream {
		t.Error("JetStream should be false for a server started without it")
	}
}

func TestDialInfoReportsJetStream(t *testing.T) {
	ns := startTestNATS(t, &natsserver.Options{
		ServerName: "js-test",
		JetStream:  true,
		StoreDir:   t.TempDir(),
	})

	info, err := DialInfo(context.Background(), ns.ClientURL(), 3*time.Second)
	if err != nil {
		t.Fatalf("DialInfo failed: %v", err)
	}
	if !info.JetStream {
		t.Error("JetStream should be true for a JetStream-enabled server")
	}
}

func TestDialInfoOnNothingListening(t *testing.T) {
	// Bind and immediately release, so the port is almost certainly free.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not pick a free port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	if _, err := DialInfo(context.Background(), "nats://"+addr, time.Second); err == nil {
		t.Fatal("expected an error dialling a closed port")
	}
}

// A non-NATS listener (nats.server_url pointed at the 8222 monitoring port, say)
// must produce a diagnosis, not a JSON parse error.
func TestDialInfoOnANonNATSListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
		conn.Close()
	}()

	_, err = DialInfo(context.Background(), "nats://"+ln.Addr().String(), 2*time.Second)
	if err == nil {
		t.Fatal("expected an error for a non-NATS listener")
	}
	if !strings.Contains(err.Error(), "NATS INFO greeting") {
		t.Errorf("error should say it was not a NATS server, got: %v", err)
	}
}

// A malformed credential fails CheckCreds and is named as the problem.
func TestCheckCredsRejectsGarbage(t *testing.T) {
	ns := startTestNATS(t, nil)

	err := CheckCreds(ns.ClientURL(), "not a creds file", time.Second)
	if err == nil {
		t.Fatal("expected an error for an unparseable credential")
	}
	if !strings.Contains(err.Error(), "not a valid .creds file") {
		t.Errorf("error should name the credential as the problem, got: %v", err)
	}
}

// mintOperatorWorld builds a complete operator / $SYS / account / user set, the
// shape pb-nats seeds centrally, and returns the operator JWT, the $SYS account
// public key and JWT, and a usable .creds file for a user in that account.
func mintOperatorWorld(t *testing.T) (opJWT, sysPub, sysJWT, creds string) {
	t.Helper()

	okp, err := nkeys.CreateOperator()
	if err != nil {
		t.Fatalf("create operator key: %v", err)
	}
	opub, _ := okp.PublicKey()

	sysKP, _ := nkeys.CreateAccount()
	sysPub, _ = sysKP.PublicKey()
	sysJWT, err = jwt.NewAccountClaims(sysPub).Encode(okp)
	if err != nil {
		t.Fatalf("encode system account: %v", err)
	}

	oc := jwt.NewOperatorClaims(opub)
	oc.Name = "stone-age.io"
	oc.SystemAccount = sysPub
	opJWT, err = oc.Encode(okp)
	if err != nil {
		t.Fatalf("encode operator: %v", err)
	}

	ukp, _ := nkeys.CreateUser()
	upub, _ := ukp.PublicKey()
	uc := jwt.NewUserClaims(upub)
	uc.Name = "sys"
	userJWT, err := uc.Encode(sysKP)
	if err != nil {
		t.Fatalf("encode user: %v", err)
	}
	seed, _ := ukp.Seed()
	raw, err := jwt.FormatUserConfig(userJWT, seed)
	if err != nil {
		t.Fatalf("FormatUserConfig: %v", err)
	}
	return opJWT, sysPub, sysJWT, string(raw)
}

// startOperatorNATS runs a real nats-server in operator mode, trusting exactly
// one operator, with the accounts preloaded into a MEMORY resolver, as
// `nats export` and the agent's buildLeafConf produce.
func startOperatorNATS(t *testing.T, opJWT, sysPub, sysJWT string) *natsserver.Server {
	t.Helper()
	conf := "operator: \"" + opJWT + "\"\n" +
		"system_account: " + sysPub + "\n" +
		"resolver: MEMORY\n" +
		"resolver_preload: {\n  " + sysPub + ": \"" + sysJWT + "\"\n}\n"

	path := filepath.Join(t.TempDir(), "nats.conf")
	if err := os.WriteFile(path, []byte(conf), 0o644); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	opts, err := natsserver.ProcessConfigFile(path)
	if err != nil {
		t.Fatalf("nats-server rejected the test config: %v", err)
	}
	return startTestNATS(t, opts)
}

// The scenario nats_trust exists for: the server's operator is not this
// database's. A real operator-mode server, because the thing asserted is the
// server's trust decision, and a mock asserts only what we believe it to be.
func TestCheckCredsRejectsAnUntrustedOperator(t *testing.T) {
	// World A runs the server.
	opJWT, sysPub, sysJWT, _ := mintOperatorWorld(t)
	ns := startOperatorNATS(t, opJWT, sysPub, sysJWT)

	// World B is the database's operator, re-seeded without a new nats.conf.
	_, _, _, otherCreds := mintOperatorWorld(t)

	err := CheckCreds(ns.ClientURL(), otherCreds, 5*time.Second)
	if err == nil {
		t.Fatal("a server must reject a credential signed by an operator it does not trust")
	}
	if strings.Contains(err.Error(), "not a valid .creds file") {
		t.Errorf("an untrusted credential must not be reported as malformed — "+
			"the two have completely different fixes: %v", err)
	}
}

// The paired "can": without it, a CheckCreds that rejects everything passes the
// test above.
func TestCheckCredsAcceptsATrustedOperator(t *testing.T) {
	opJWT, sysPub, sysJWT, creds := mintOperatorWorld(t)
	ns := startOperatorNATS(t, opJWT, sysPub, sysJWT)

	if err := CheckCreds(ns.ClientURL(), creds, 5*time.Second); err != nil {
		t.Fatalf("the server should accept a credential from the operator it trusts: %v", err)
	}
}
