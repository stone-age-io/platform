package leafsync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/prometheus/client_golang/prometheus"

	"platform/internal/health"
)

// runCheck executes one registered check by name against a state.
func runCheck(t *testing.T, state *syncState, name string) health.Result {
	t.Helper()
	var reg health.Registry
	registerLeafChecks(&reg, state)
	rep := reg.Run(context.Background(), "test", 0)
	for _, c := range rep.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no check named %q", name)
	return health.Result{}
}

func testState(t *testing.T, monitorURL string, collections ...string) *syncState {
	t.Helper()
	return newSyncState(&Config{
		SyncInterval: 30 * time.Second,
		MonitorURL:   monitorURL,
	}, collections)
}

// monitorStub stands in for the local NATS server's loopback monitoring port —
// the `http:` line buildLeafConf writes. It is what lets the edge read its own
// server's state without ever holding a $SYS identity.
func monitorStub(t *testing.T, leafz leafzResponse) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/leafz":
			_ = json.NewEncoder(w).Encode(leafz)
		case "/varz":
			_, _ = w.Write([]byte(`{"server_name":"edge-s01","connections":7,
				"jetstream":{"stats":{"memory":100,"storage":2048}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// Freshness is about the loop still turning, not about the last cycle
// succeeding. A wedged sync loop freezes every number downstream — the mirror,
// the heartbeat, the record gauges — and all of them keep reporting their last
// good value, so nothing else can detect it.
func TestSyncFreshnessCheck(t *testing.T) {
	t.Run("no cycles yet is a warning, not a failure", func(t *testing.T) {
		state := testState(t, "", "things")
		res := runCheck(t, state, "sync_freshness")
		if res.State != health.StateWarn {
			t.Errorf("state = %q, want warn during startup", res.State)
		}
	})

	t.Run("a recent cycle is ok", func(t *testing.T) {
		state := testState(t, "", "things")
		state.recordCycle(map[string]int{"things": 3}, nil, time.Second)
		res := runCheck(t, state, "sync_freshness")
		if res.State != health.StateOK {
			t.Errorf("state = %q (%s), want ok", res.State, res.Detail)
		}
	})

	t.Run("a stale cycle fails", func(t *testing.T) {
		state := testState(t, "", "things")
		state.recordCycle(map[string]int{"things": 3}, nil, time.Second)
		// Older than the three-interval budget.
		state.mu.Lock()
		state.lastCycle = time.Now().Add(-5 * time.Minute)
		state.mu.Unlock()

		res := runCheck(t, state, "sync_freshness")
		if res.State != health.StateFail {
			t.Errorf("state = %q, want fail for a frozen sync loop", res.State)
		}
		if res.Fix == "" {
			t.Error("a failing check must tell the operator what to do")
		}
	})

	// A leaf node with nothing to mirror is a legitimate configuration, and it
	// never advances lastCycle in a meaningful way. Reporting it as ok would
	// claim a check ran; reporting it as failed would page someone over a
	// deliberate setting.
	t.Run("no collections configured is skipped", func(t *testing.T) {
		state := testState(t, "")
		state.recordCycle(map[string]int{}, nil, time.Second)
		state.mu.Lock()
		state.lastCycle = time.Now().Add(-5 * time.Minute)
		state.mu.Unlock()

		res := runCheck(t, state, "sync_freshness")
		if res.State != health.StateSkipped {
			t.Errorf("state = %q, want skipped", res.State)
		}
	})
}

// syncAll is fail-soft by design: a collection that errors keeps its existing
// mirror and retries next interval, so the site goes on working with slightly
// stale config. That is a warning, not an unready process — failing here would
// have an orchestrator restart a functioning edge box over a transient hub
// error.
func TestSyncErrorsWarnRatherThanFail(t *testing.T) {
	state := testState(t, "", "things")
	state.recordCycle(map[string]int{"locations": 2}, []string{"things: kv bucket: timeout"}, time.Second)

	res := runCheck(t, state, "sync_errors")
	if res.State != health.StateWarn {
		t.Fatalf("state = %q, want warn", res.State)
	}
	if !strings.Contains(res.Detail, "things") {
		t.Errorf("detail should name the failing collection, got %q", res.Detail)
	}
}

// The uplink check is the one that distinguishes "this site is islanded" from
// "this site is fine", and it is only answerable on the box.
func TestHubUplinkCheck(t *testing.T) {
	t.Run("an outbound leaf connection is ok", func(t *testing.T) {
		url := monitorStub(t, leafzResponse{
			Leafs: []leafInfo{{Name: "hub", IsSpoke: true, RTT: "12ms"}},
		})
		res := runCheck(t, testState(t, url), "hub_uplink")
		if res.State != health.StateOK {
			t.Errorf("state = %q (%s), want ok", res.State, res.Detail)
		}
	})

	// A leaf connection INTO this server is not the uplink. Counting any leaf
	// connection would report a healthy uplink on a site that is islanded but
	// happens to have something bridging in to it.
	t.Run("an inbound leaf connection is not an uplink", func(t *testing.T) {
		url := monitorStub(t, leafzResponse{
			Leafs: []leafInfo{{Name: "some-device", IsSpoke: false}},
		})
		res := runCheck(t, testState(t, url), "hub_uplink")
		if res.State != health.StateWarn {
			t.Errorf("state = %q, want warn — inbound leafs are not the hub uplink", res.State)
		}
	})

	// An islanded site is a warning, not a failure: local NATS still works and
	// devices keep running against the mirrored config. That autonomy is the
	// entire reason a leaf node exists, so reporting it as unready would invert
	// the design.
	t.Run("no uplink warns rather than fails", func(t *testing.T) {
		url := monitorStub(t, leafzResponse{})
		res := runCheck(t, testState(t, url), "hub_uplink")
		if res.State != health.StateWarn {
			t.Errorf("state = %q, want warn", res.State)
		}
		if !strings.Contains(res.Fix, "islanded") {
			t.Errorf("the fix text should explain what still works, got %q", res.Fix)
		}
	})

	// An unreachable monitoring port means we could not look, which is not the
	// same as looking and seeing no uplink.
	t.Run("an unreachable monitor endpoint is skipped", func(t *testing.T) {
		res := runCheck(t, testState(t, "http://127.0.0.1:1"), "hub_uplink")
		if res.State != health.StateSkipped {
			t.Errorf("state = %q, want skipped", res.State)
		}
	})

	t.Run("an unset monitor url is skipped", func(t *testing.T) {
		res := runCheck(t, testState(t, ""), "hub_uplink")
		if res.State != health.StateSkipped {
			t.Errorf("state = %q, want skipped", res.State)
		}
	})
}

// The local NATS check asks the connection whether it is connected, rather than
// inferring from the last cycle: a sync loop with nothing to do looks exactly
// like one whose connection dropped.
func TestNatsLocalCheck(t *testing.T) {
	t.Run("no connection fails", func(t *testing.T) {
		res := runCheck(t, testState(t, ""), "nats_local")
		if res.State != health.StateFail {
			t.Errorf("state = %q, want fail", res.State)
		}
	})

	t.Run("a live connection is ok", func(t *testing.T) {
		ns, err := natsserver.NewServer(&natsserver.Options{Port: -1, NoLog: true, NoSigs: true})
		if err != nil {
			t.Fatalf("create server: %v", err)
		}
		go ns.Start()
		if !ns.ReadyForConnections(10 * time.Second) {
			t.Fatal("test server never became ready")
		}
		defer func() { ns.Shutdown(); ns.WaitForShutdown() }()

		nc, err := nats.Connect(ns.ClientURL())
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
		defer nc.Close()

		state := testState(t, "")
		state.setConn(nc)

		if res := runCheck(t, state, "nats_local"); res.State != health.StateOK {
			t.Errorf("state = %q (%s), want ok", res.State, res.Detail)
		}
	})
}

// The collector must publish one series per mirrored collection, so a site with
// a stale or empty collection is visible per-collection rather than as one
// aggregate.
func TestLeafCollectorPublishesPerCollectionCounts(t *testing.T) {
	state := testState(t, "", "things", "locations")
	state.recordCycle(map[string]int{"things": 12, "locations": 4}, nil, 250*time.Millisecond)

	reg := prometheus.NewRegistry()
	reg.MustRegister(&leafCollector{state: state})

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	got := map[string]float64{}
	for _, fam := range families {
		if fam.GetName() != "leaf_sync_mirrored_records" {
			continue
		}
		for _, m := range fam.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "collection" {
					got[l.GetValue()] = m.GetGauge().GetValue()
				}
			}
		}
	}
	if got["things"] != 12 || got["locations"] != 4 {
		t.Errorf("mirrored records = %v, want things=12 locations=4", got)
	}
}

// When the monitoring endpoint is unreachable the server-derived series are
// omitted entirely rather than reported as zero. Zero would claim an islanded
// site with no devices attached, which is a far more alarming statement than
// "this was not scraped".
func TestLeafCollectorOmitsServerSeriesWhenMonitorIsDown(t *testing.T) {
	state := testState(t, "http://127.0.0.1:1")
	state.recordCycle(map[string]int{"things": 1}, nil, time.Millisecond)

	reg := prometheus.NewRegistry()
	reg.MustRegister(&leafCollector{state: state})

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, fam := range families {
		switch fam.GetName() {
		case "leaf_sync_hub_uplink_connected", "leaf_sync_nats_connections", "leaf_sync_jetstream_bytes":
			t.Errorf("%s was published despite the monitoring endpoint being unreachable", fam.GetName())
		}
	}
}

// The listener is opt-in, but the checks are not: an edge box with no
// monitoring port still runs them and still logs a readiness transition.
func TestObservabilityListenerIsOptIn(t *testing.T) {
	state := testState(t, "")
	obs := newObservability(&Config{ReadinessInterval: time.Hour}, state)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	obs.Start(ctx)

	if obs.srv != nil {
		t.Error("no listener should be created when observability.addr is empty")
	}
	// Give the prober's first (synchronous-then-backgrounded) run a moment.
	deadline := time.Now().Add(2 * time.Second)
	for obs.prober.Snapshot() == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if obs.prober.Snapshot() == nil {
		t.Error("checks must run even with no listener configured")
	}
}

// With an address configured, both endpoints answer.
func TestObservabilityServesReadyAndMetrics(t *testing.T) {
	state := testState(t, "")
	state.recordCycle(map[string]int{"things": 1}, nil, time.Millisecond)

	obs := newObservability(&Config{
		ReadinessInterval: time.Hour,
		ObserveAddr:       "127.0.0.1:0",
	}, state)

	// Port 0 means the kernel picks; capture what it picked by listening here.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	obs.Start(ctx)
	if obs.srv == nil {
		t.Fatal("expected a listener")
	}

	// The handler is what matters; exercise it directly rather than racing the
	// goroutine that binds the socket.
	mux := obs.srv.Handler

	for _, path := range []string{"/ready", "/metrics"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		// /ready answers 503 here (nats_local fails with no connection), which
		// is a legitimate answer — what matters is that it is not a 404.
		if rec.Code == http.StatusNotFound {
			t.Errorf("%s returned 404", path)
		}
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(rec.Body.String(), "leaf_sync_") {
		t.Error("/metrics did not expose the leaf_sync namespace")
	}
}
