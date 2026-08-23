package leafsync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"

	"platform/internal/health"
	"platform/internal/metrics"
	"platform/internal/version"
)

// Observability is leaf-sync's readiness and metrics surface.
//
// WHY THE EDGE GETS ITS OWN, RATHER THAN REPORTING UP. Everything that says
// whether a site is actually healthy — is the config mirror fresh, is the
// uplink to the hub attached, is JetStream filling the disk — lives inside the
// organization's NATS account and on this box. The Control Plane holds the NATS
// operator and the $SYS account and has no credential inside any tenant's
// account, so it cannot see any of it; what it can count is how many leaf nodes
// are *configured*, which is not the same question.
//
// The `leaf_status` KV heartbeat (heartbeat.go) already carries a summary of
// this up to the hub for the console to render. That is a product feature for
// the tenant, not a monitoring channel: it is best-effort, it is one key per
// site with a fixed shape, and — being delivered over the very link that breaks
// — it says nothing during exactly the outage you want to know about. This
// endpoint is scraped locally and keeps answering when the WAN is down.
type Observability struct {
	prober  *health.Prober
	metrics *metrics.Set
	addr    string
	token   string
	srv     *http.Server
}

// syncState is what the sync loop knows about itself, published for the checks
// and collectors to read. Written once per cycle, read on probe and on scrape.
type syncState struct {
	mu sync.RWMutex

	interval     time.Duration
	cycles       uint64
	lastCycle    time.Time
	lastDuration time.Duration
	synced       map[string]int
	errs         []string
	collections  []string

	// nc is the live connection to the local leaf. Held so a check can ask it
	// whether it is currently connected rather than inferring from the last
	// cycle's outcome — a sync loop that has nothing to do looks identical to
	// one whose connection dropped.
	nc *nats.Conn

	// monitorURL is the local NATS monitoring endpoint (`http:` in
	// nats-leaf.conf). Loopback and unauthenticated by design; see the comment
	// on buildLeafConf.
	monitorURL string
}

func newSyncState(cfg *Config, collections []string) *syncState {
	return &syncState{
		interval:    cfg.SyncInterval,
		collections: collections,
		synced:      map[string]int{},
		monitorURL:  cfg.MonitorURL,
	}
}

func (s *syncState) recordCycle(synced map[string]int, errs []string, took time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cycles++
	s.lastCycle = time.Now()
	s.lastDuration = took
	s.synced = synced
	s.errs = errs
}

func (s *syncState) setConn(nc *nats.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nc = nc
}

// snapshot copies the mutable fields under one lock, so a caller reading
// several of them never sees a half-updated cycle.
func (s *syncState) snapshot() syncSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	synced := make(map[string]int, len(s.synced))
	for k, v := range s.synced {
		synced[k] = v
	}
	return syncSnapshot{
		cycles:       s.cycles,
		lastCycle:    s.lastCycle,
		lastDuration: s.lastDuration,
		synced:       synced,
		errs:         append([]string(nil), s.errs...),
		collections:  s.collections,
		interval:     s.interval,
		connected:    s.nc != nil && s.nc.IsConnected(),
	}
}

type syncSnapshot struct {
	cycles       uint64
	lastCycle    time.Time
	lastDuration time.Duration
	synced       map[string]int
	errs         []string
	collections  []string
	interval     time.Duration
	connected    bool
}

// newObservability builds the edge's readiness registry and metrics.
func newObservability(cfg *Config, state *syncState) *Observability {
	var reg health.Registry
	registerLeafChecks(&reg, state)

	set := metrics.New("leaf_sync", version.Version)
	set.Registry.MustRegister(&leafCollector{state: state})

	prober := health.NewProber(&reg, version.Version, cfg.ReadinessInterval, health.DefaultProbeTimeout)
	set.Bind(prober)
	prober.OnChange(func(rep *health.Report) { health.LogReport("leaf-sync readiness", rep) })

	return &Observability{
		prober:  prober,
		metrics: set,
		addr:    cfg.ObserveAddr,
		token:   cfg.MetricsToken,
	}
}

// Start begins probing and, when an address is configured, serves /ready and
// /metrics on it.
//
// The listener is opt-in and the prober is not. Running the checks always costs
// nothing and means the log carries a readiness transition even on a box with
// no monitoring; opening a port on an edge appliance is a decision someone
// should have made on purpose.
func (o *Observability) Start(ctx context.Context) {
	go o.prober.Start(ctx)

	if o.addr == "" {
		return
	}

	mux := http.NewServeMux()
	mux.Handle("/ready", o.prober.Handler())
	mux.Handle("/metrics", o.metrics.Handler(o.token))
	// A bare GET on the root is what someone typing the address into a browser
	// does. Point them at the two real paths rather than 404ing.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "leaf-sync: try /ready or /metrics", http.StatusNotFound)
	})

	o.srv = &http.Server{
		Addr:              o.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ln, err := net.Listen("tcp", o.addr)
	if err != nil {
		// Never fatal. A monitoring port that cannot bind must not stop an edge
		// site from syncing its config — that would make observability a
		// dependency of the thing it observes.
		log.Printf("⚠️ leaf-sync: cannot serve /ready and /metrics on %s: %v", o.addr, err)
		return
	}
	log.Printf("leaf-sync: readiness + metrics on http://%s/ready and /metrics", o.addr)

	go func() {
		if err := o.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("⚠️ leaf-sync: observability listener stopped: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = o.srv.Shutdown(shutdownCtx)
	}()
}

// ------------------------------------------------------------------ checks

func registerLeafChecks(reg *health.Registry, state *syncState) {
	reg.Register("nats_local", func(ctx context.Context) health.Result {
		snap := state.snapshot()
		if !snap.connected {
			return health.Fail(
				"not connected to the local NATS leaf",
				"Check that the leaf node is running and that nats.local_url and nats.creds_file are correct. "+
					"With `run --nats` the server is in this process, so this failing means it did not start.",
			)
		}
		return health.OK("connected")
	})

	// Freshness rather than success/failure. A cycle that errored is reported
	// by sync_errors below; this catches the worse case where the loop stopped
	// running at all and every number downstream quietly froze.
	reg.Register("sync_freshness", func(ctx context.Context) health.Result {
		snap := state.snapshot()
		if snap.cycles == 0 {
			return health.Warn("no sync cycle has completed yet", "Normal for a few seconds after startup.")
		}
		if len(snap.collections) == 0 {
			return health.Skip("no syncable collections configured for this leaf node")
		}
		age := time.Since(snap.lastCycle)
		// Three intervals: one for the cycle that is due, one for a cycle that
		// is simply slow, and one so a single hiccup does not flap the alert.
		if limit := 3 * snap.interval; age > limit {
			return health.Fail(
				fmt.Sprintf("last sync cycle was %s ago (interval is %s)", age.Round(time.Second), snap.interval),
				"The sync loop is stuck or the process is wedged. Check the log and restart leaf-sync.",
			)
		}
		return health.OK(fmt.Sprintf("last cycle %s ago", age.Round(time.Second)))
	})

	// Warn, not fail. syncAll is fail-soft on purpose: a collection that errors
	// leaves the existing mirror in place and retries next interval, and the
	// edge keeps serving the config it already has. That is a working site with
	// a stale collection, not an unready one.
	reg.Register("sync_errors", func(ctx context.Context) health.Result {
		snap := state.snapshot()
		if len(snap.errs) == 0 {
			return health.OK(fmt.Sprintf("%d collection(s) mirrored cleanly", len(snap.synced)))
		}
		return health.Warn(
			"last cycle reported: "+strings.Join(snap.errs, "; "),
			"The existing mirror is untouched and will be retried next interval. "+
				"Persistent errors usually mean the hub is unreachable or the leaf node's PocketBase credentials changed.",
		)
	})

	// The uplink to the hub, read from the leaf server's own monitoring port.
	// This is the check that distinguishes "this site is islanded" from "this
	// site is fine" — and it is only answerable here, on the box.
	reg.Register("hub_uplink", func(ctx context.Context) health.Result {
		lz, err := fetchLeafz(ctx, state.monitorURL)
		if err != nil {
			return health.Skip("local NATS monitoring endpoint not reachable: " + err.Error())
		}
		for _, l := range lz.Leafs {
			// IsSpoke means this server dialled out — the `remotes` entry in
			// nats-leaf.conf. Leaf connections INTO this server (a device
			// bridging in) are not the uplink.
			if l.IsSpoke {
				return health.OK(fmt.Sprintf("attached to %s (rtt %s)", l.Name, l.RTT))
			}
		}
		return health.Warn(
			"no outbound leaf connection to the hub",
			"This site is islanded: local NATS still works and devices keep running against the mirrored config, "+
				"but nothing reaches the platform. Check the WAN link and nats.hub_leaf_url.",
		)
	})
}

// ---------------------------------------------------------------- collector

type leafCollector struct {
	state *syncState
}

var (
	descLeafRecords = prometheus.NewDesc(
		"leaf_sync_mirrored_records",
		"Records mirrored into local NATS KV, by collection.",
		[]string{"collection"}, nil,
	)
	descLeafCycles = prometheus.NewDesc(
		"leaf_sync_cycles_total",
		"Sync cycles completed since this agent started.",
		nil, nil,
	)
	descLeafLastCycle = prometheus.NewDesc(
		"leaf_sync_last_cycle_timestamp_seconds",
		"Unix time of the last completed sync cycle. Alert on this going stale — it means the mirror is frozen.",
		nil, nil,
	)
	descLeafCycleSeconds = prometheus.NewDesc(
		"leaf_sync_last_cycle_duration_seconds",
		"Wall time of the last sync cycle.",
		nil, nil,
	)
	descLeafErrors = prometheus.NewDesc(
		"leaf_sync_last_cycle_errors",
		"Collections that failed in the last cycle. Non-zero means a stale mirror, not a lost one.",
		nil, nil,
	)
	descLeafConnected = prometheus.NewDesc(
		"leaf_sync_nats_connected",
		"1 when the agent is connected to the local NATS leaf.",
		nil, nil,
	)
	descLeafUplink = prometheus.NewDesc(
		"leaf_sync_hub_uplink_connected",
		"1 when this leaf server holds an outbound leaf connection to the hub. 0 means the site is islanded: "+
			"local NATS keeps working, nothing reaches the platform.",
		nil, nil,
	)
	descLeafConns = prometheus.NewDesc(
		"leaf_sync_nats_connections",
		"Client connections to the local NATS leaf — the devices actually attached at this site.",
		nil, nil,
	)
	descLeafJetStream = prometheus.NewDesc(
		"leaf_sync_jetstream_bytes",
		"JetStream usage on the local leaf, by storage tier. The number to watch on an edge box with a small disk.",
		[]string{"tier"}, nil,
	)
)

func (c *leafCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descLeafRecords
	ch <- descLeafCycles
	ch <- descLeafLastCycle
	ch <- descLeafCycleSeconds
	ch <- descLeafErrors
	ch <- descLeafConnected
	ch <- descLeafUplink
	ch <- descLeafConns
	ch <- descLeafJetStream
}

func (c *leafCollector) Collect(ch chan<- prometheus.Metric) {
	snap := c.state.snapshot()

	g := func(d *prometheus.Desc, v float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, labels...)
	}

	ch <- prometheus.MustNewConstMetric(descLeafCycles, prometheus.CounterValue, float64(snap.cycles))
	g(descLeafCycleSeconds, snap.lastDuration.Seconds())
	g(descLeafErrors, float64(len(snap.errs)))
	g(descLeafConnected, boolGauge(snap.connected))
	if !snap.lastCycle.IsZero() {
		g(descLeafLastCycle, float64(snap.lastCycle.Unix()))
	}
	for col, n := range snap.synced {
		g(descLeafRecords, float64(n), col)
	}

	// The local server's own view. Skipped silently when the monitoring
	// endpoint is not reachable — emitting zeros would report an islanded site
	// with no devices, which is a much more alarming claim than "not scraped".
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if vz, err := fetchVarz(ctx, c.state.monitorURL); err == nil {
		g(descLeafConns, float64(vz.Connections))
		if vz.JetStream.Stats != nil {
			g(descLeafJetStream, float64(vz.JetStream.Stats.Memory), "memory")
			g(descLeafJetStream, float64(vz.JetStream.Stats.Store), "file")
		}
	}
	if lz, err := fetchLeafz(ctx, c.state.monitorURL); err == nil {
		uplink := false
		for _, l := range lz.Leafs {
			if l.IsSpoke {
				uplink = true
				break
			}
		}
		g(descLeafUplink, boolGauge(uplink))
	}
}

func boolGauge(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// ------------------------------------------------- local NATS monitoring

// The monitoring endpoint is the `http:` line buildLeafConf writes, bound to
// loopback. It needs no credential precisely because it is loopback-only, which
// is why the edge can read its own server's state without ever holding a $SYS
// identity.

// Only the fields actually reported are declared. encoding/json ignores the
// rest of the payload, and a field parsed but never read is a promise to a
// reader that something uses it.
type varzResponse struct {
	Connections int `json:"connections"`
	JetStream   struct {
		Stats *struct {
			Memory uint64 `json:"memory"`
			Store  uint64 `json:"storage"`
		} `json:"stats"`
	} `json:"jetstream"`
}

type leafzResponse struct {
	Leafs []leafInfo `json:"leafs"`
}

type leafInfo struct {
	Name    string `json:"name"`
	IsSpoke bool   `json:"is_spoke"`
	RTT     string `json:"rtt"`
}

func fetchVarz(ctx context.Context, base string) (*varzResponse, error) {
	var out varzResponse
	return &out, fetchJSON(ctx, base, "varz", &out)
}

func fetchLeafz(ctx context.Context, base string) (*leafzResponse, error) {
	var out leafzResponse
	return &out, fetchJSON(ctx, base, "leafz", &out)
}

var monitorClient = &http.Client{Timeout: 2 * time.Second}

func fetchJSON(ctx context.Context, base, path string, into any) error {
	if base == "" {
		return fmt.Errorf("nats.monitor_url is not set")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(base, "/")+"/"+path, nil)
	if err != nil {
		return err
	}
	resp, err := monitorClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}
