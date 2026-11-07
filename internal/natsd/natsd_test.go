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

// Stats reports nothing when there is no embedded server, so the caller omits
// the series rather than publish zeros that read as a quiet bus.
func TestStatsOnNoServer(t *testing.T) {
	var nilSrv *Server
	if _, ok := nilSrv.Stats(); ok {
		t.Error("Stats on a nil Server should report ok == false")
	}
	if _, ok := (&Server{}).Stats(); ok {
		t.Error("Stats on a zero Server should report ok == false")
	}
}
