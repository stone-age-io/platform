package natsd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConf(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nats.conf")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	return path
}

// The embedded server clusters, and this test is the reason the package comment
// no longer claims otherwise.
//
// Start() hands the parsed config straight to nats-server: it sets NoSigs and
// nothing else, so every directive in the file — `cluster` included — is
// honoured exactly as `nats-server -c nats.conf` would honour it. That is the
// package's stated design ("no embedded-only mode, no options derived in Go"),
// and a clustered Control Plane peered with external nodes is a supported
// topology rather than a thing to be talked out of.
//
// It also justifies Stats().Routes existing. A route count that could only ever
// be zero would be a metric nobody can act on; this proves it is not.
//
// Every port is -1, so the kernel assigns it at bind time and nothing can take
// it first. This used to reserve four ports up front by opening and closing a
// listener, which raced: a parallel test package bound one in the gap and node
// A failed with "address already in use". Only B has to name a route; one
// direction is enough to form a cluster, and B dials A's port once A holds it.
// The empty clientURL skips Start's port check, which needs a fixed port.
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
		stats, ok := srvA.Stats()
		if ok && stats.Routes > 0 {
			return // clustered
		}
		if time.Now().After(deadline) {
			routes := -1
			if ok {
				routes = stats.Routes
			}
			t.Fatalf("node A never peered with node B (Routes = %d)", routes)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Stats reports nothing at all when there is no embedded server, so the caller
// can omit the series rather than publish zeros that read as a quiet bus.
func TestStatsOnNoServer(t *testing.T) {
	var nilSrv *Server
	if _, ok := nilSrv.Stats(); ok {
		t.Error("Stats on a nil Server should report ok == false")
	}
	if _, ok := (&Server{}).Stats(); ok {
		t.Error("Stats on a zero Server should report ok == false")
	}
}
