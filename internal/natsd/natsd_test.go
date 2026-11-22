package natsd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func writeConf(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nats.conf")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	return path
}

// The embedded server honours a `cluster` block like any other directive, so a
// Control Plane can peer with external nodes, and Stats().Routes is non-zero.
//
// Every port is -1 so the kernel assigns it at bind time; reserving ports up
// front raced with parallel test packages. Only B names a route, which is
// enough to form a cluster. The empty clientURL skips Start's port check.
func TestEmbeddedServerClustersWithAPeer(t *testing.T) {
	conf := func(name, routes string) string {
		return fmt.Sprintf(`
server_name: %q
host: "127.0.0.1"
port: -1
cluster {
  name: "stone-age-test"
  host: "127.0.0.1"
  port: -1
  routes: [%s]
}
`, name, routes)
	}

	srvA, err := Start(writeConf(t, conf("node-a", "")), "", "nats.server_url")
	if err != nil {
		t.Fatalf("node A refused to start with a cluster block: %v", err)
	}
	defer srvA.Stop()

	routeA := srvA.ns.ClusterAddr()
	if routeA == nil {
		t.Fatal("node A has no cluster listener")
	}

	srvB, err := Start(
		writeConf(t, conf("node-b", fmt.Sprintf(`"nats://127.0.0.1:%d"`, routeA.Port))),
		"",
		"nats.server_url",
	)
	if err != nil {
		t.Fatalf("node B refused to start with a cluster block: %v", err)
	}
	defer srvB.Stop()

	// Route establishment is asynchronous; poll rather than sleep a fixed span.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if srvA.ns.NumRoutes() > 0 {
			return // clustered
		}
		if time.Now().After(deadline) {
			t.Fatal("node A never peered with node B")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// reloadConf is a single-node config whose client port is fixed after the first
// start, so a reload can be checked against the same listener.
func reloadConf(port, maxPayload int) string {
	return fmt.Sprintf(`
server_name: "reload-test"
host: "127.0.0.1"
port: %d
max_payload: %d
`, port, maxPayload)
}

func clientMaxPayload(t *testing.T, url string) int64 {
	t.Helper()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect %s: %v", url, err)
	}
	defer nc.Close()
	return nc.MaxPayload()
}

// ADR 0001 step 3: an edited nats.conf applies without a restart. A change
// nats-server cannot make live, and a file that does not parse, are refused,
// and the server keeps its config and its listener.
func TestReloadAppliesTheEditedConfig(t *testing.T) {
	path := writeConf(t, reloadConf(-1, 1<<20))
	srv, err := Start(path, "", "nats.server_url")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()
	url := srv.ns.ClientURL()
	port := srv.ns.Addr().(*net.TCPAddr).Port

	// Pin the port the kernel chose, so only max_payload differs.
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(reloadConf(port, 1<<20))
	if err := srv.Reload(); err != nil {
		t.Fatalf("reload of an unchanged config: %v", err)
	}

	write(reloadConf(port, 2<<20))
	if err := srv.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := clientMaxPayload(t, url); got != 2<<20 {
		t.Fatalf("max_payload after reload = %d, want %d", got, 2<<20)
	}

	// The client port cannot change live: refused, and the old listener stays.
	write(reloadConf(port+1, 2<<20))
	err = srv.Reload()
	if err == nil || !strings.Contains(err.Error(), "keeps its current config") {
		t.Fatalf("reload with a new port: %v, want a refusal", err)
	}
	t.Logf("refused as expected: %v", err)
	if got := clientMaxPayload(t, url); got != 2<<20 {
		t.Fatalf("max_payload after a refused reload = %d", got)
	}

	// A file that does not parse is refused the same way.
	write("port: {")
	if err := srv.Reload(); err == nil {
		t.Fatal("reload of a broken file succeeded")
	}
	clientMaxPayload(t, url)

	// Fixing the file makes the next reload succeed.
	write(reloadConf(port, 1<<20))
	if err := srv.Reload(); err != nil {
		t.Fatalf("reload after fixing the file: %v", err)
	}
	if got := clientMaxPayload(t, url); got != 1<<20 {
		t.Fatalf("max_payload after the fixed reload = %d", got)
	}
}
