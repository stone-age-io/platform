package leafsync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nats-io/nats.go"
)

// TestLocalConnectRetriesForever pins the option whose absence turned leaf-sync
// into a zombie about two minutes after the local bus restarted.
//
// nats.go's default MaxReconnect is 60, after which the connection is closed
// permanently -- while Run keeps looping, failing every write, with no reconnect
// and no exit for a supervisor to act on. The options are built by a named
// function purely so this assertion is possible; an inline argument list to
// nats.Connect cannot be inspected.
//
// Asserting on the resolved nats.Options rather than on source text means a
// later edit that reorders or replaces these options is still covered.
func TestLocalConnectRetriesForever(t *testing.T) {
	opts := nats.GetDefaultOptions()
	for _, apply := range localConnectOptions(&Config{CredsFile: writeStubCreds(t)}) {
		if err := apply(&opts); err != nil {
			t.Fatalf("applying a connect option failed: %v", err)
		}
	}

	if opts.MaxReconnect != -1 {
		t.Errorf("MaxReconnect = %d, want -1 (infinite). A finite value means the "+
			"agent stops reconnecting after a local NATS restart and then spins "+
			"forever without syncing.", opts.MaxReconnect)
	}
	if opts.ReconnectWait <= 0 {
		t.Errorf("ReconnectWait = %v, want a positive backoff", opts.ReconnectWait)
	}

	// A disconnect that logs nothing is how this went unnoticed for so long: the
	// only symptom was silence on a box nobody was scraping.
	if opts.DisconnectedErrCB == nil {
		t.Error("no DisconnectErrHandler set; a silent disconnect is undiagnosable")
	}
	if opts.ReconnectedCB == nil {
		t.Error("no ReconnectHandler set; recovery should be visible in the log")
	}
	if opts.ClosedCB == nil {
		t.Error("no ClosedHandler set; a permanently closed connection must say so")
	}
}

// TestLocalConnectStillFailsFastOnFirstDial guards the other half of the
// asymmetry. RetryOnFailedConnect must stay OFF: without --nats the local bus is
// a separate process that should already be running, and a hard error at startup
// is how an operator finds out the creds path or URL is wrong. Turning it on
// would also hand newKVWriter a connection that is not up yet.
func TestLocalConnectStillFailsFastOnFirstDial(t *testing.T) {
	opts := nats.GetDefaultOptions()
	for _, apply := range localConnectOptions(&Config{CredsFile: writeStubCreds(t)}) {
		if err := apply(&opts); err != nil {
			t.Fatalf("applying a connect option failed: %v", err)
		}
	}

	if opts.RetryOnFailedConnect {
		t.Error("RetryOnFailedConnect is on: the initial dial should fail fast so a " +
			"bad creds path or URL is reported at startup rather than retried silently")
	}
}

// writeStubCreds writes a credentials file for the option-application above.
// nats.UserCredentials opens the path when the option is applied, so the file
// has to exist -- but nothing here dials a server, so the contents are never
// parsed and a placeholder is enough.
func writeStubCreds(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "edge.creds")
	if err := os.WriteFile(path, []byte("-----BEGIN NATS USER JWT-----\nstub\n------END NATS USER JWT------\n"), 0o600); err != nil {
		t.Fatalf("could not write stub creds: %v", err)
	}
	return path
}
