//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// ADR 0001 step 3: SIGHUP to `serve --nats` applies an edited nats.conf to the
// embedded server without a restart. An open connection stays up, and an
// account pushed over $SYS before the reload still connects after it, because
// the full resolver keeps account JWTs in its directory.
func TestSIGHUPReloadsTheEmbeddedNATSConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no SIGHUP")
	}
	tn := newTenant(t)
	_, creds := tn.natsUser(t, tn.role(t, []string{"probe.>"}, []string{"_INBOX.>"}))
	before := connect(t, creds)
	if got := before.MaxPayload(); got != 1<<20 {
		t.Fatalf("max_payload before the reload = %d, want the default %d", got, 1<<20)
	}

	conf := filepath.Join(platform.Dir, "nats-config", "nats.conf")
	original, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	hup := func() {
		t.Helper()
		if err := platform.cmd.Process.Signal(syscall.SIGHUP); err != nil {
			t.Fatal(err)
		}
	}
	// Every test shares this server: put the file back and reload it.
	t.Cleanup(func() {
		if err := os.WriteFile(conf, original, 0o644); err != nil {
			t.Error(err)
			return
		}
		_ = platform.cmd.Process.Signal(syscall.SIGHUP)
	})

	edited := append(append([]byte{}, original...), []byte("\nmax_payload: 2MB\n")...)
	if err := os.WriteFile(conf, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	hup()

	eventually(t, 10*time.Second, func() error {
		nc, err := dial(creds)
		if err != nil {
			return err
		}
		defer nc.Close()
		if got := nc.MaxPayload(); got != 2<<20 {
			return fmt.Errorf("max_payload = %d, want %d", got, 2<<20)
		}
		return nil
	})
	if before.IsClosed() || !before.IsConnected() {
		t.Fatal("the reload dropped a connection that was open before it")
	}
	if err := before.Publish("probe.after-reload", []byte("1")); err != nil {
		t.Fatal(err)
	}
	if err := before.Flush(); err != nil {
		t.Fatalf("flush after the reload: %v", err)
	}
}
