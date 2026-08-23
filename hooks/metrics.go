package hooks

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/prometheus/client_golang/prometheus"

	"platform/internal/metrics"
	"platform/internal/natsd"
)

// registerPlatformMetrics adds the Control Plane's own collectors to a set.
//
// WHAT THESE NUMBERS ARE. Everything here is either this process's own HTTP
// traffic, a row count in its own SQLite database, or — when `--nats` is on —
// the embedded NATS server's own counters. That is the whole of what the
// Control Plane can observe first-hand.
//
// WHAT THEY ARE NOT. `stone_age_records{collection="leaf_nodes"}` counts leaf
// nodes CONFIGURED, not leaf nodes online. Liveness lives in the `leaf_status`
// KV bucket inside each organization's NATS account, which this process holds
// no credential for; it is read by the console (the browser connects with the
// logged-in user's own in-account credential) and exported by leaf-sync itself
// on the edge. Alerting on this series as though it were availability would
// produce an alert that can never fire.
func registerPlatformMetrics(app *pocketbase.PocketBase, set *metrics.Set, opts ObservabilityOptions) {
	set.Registry.MustRegister(&dbCollector{app: app, opts: opts})
	if opts.EmbeddedNATS != nil {
		set.Registry.MustRegister(&embeddedNATSCollector{server: opts.EmbeddedNATS})
	}
}

// --------------------------------------------------------------- database

// dbCollector counts rows at scrape time.
//
// Scrape time rather than on the readiness prober's tick, unlike the check
// gauges: these are plain local COUNT queries with no network in them, so
// doing the work only when someone actually asks is both cheap and honest. The
// readiness checks go the other way because they dial NATS, and that must not
// sit on the scrape path.
type dbCollector struct {
	app  core.App
	opts ObservabilityOptions
}

var (
	descRecords = prometheus.NewDesc(
		"stone_age_records",
		"Rows in a platform collection. This is inventory CONFIGURED, not anything online: "+
			"leaf-node and device liveness lives inside an organization's NATS account, which this process cannot read.",
		[]string{"collection"}, nil,
	)
	descInactive = prometheus.NewDesc(
		"stone_age_inactive_records",
		"Rows with active = false: devices and leaf nodes that have been decommissioned.",
		[]string{"collection"}, nil,
	)
	descRevokedUsers = prometheus.NewDesc(
		"stone_age_nats_users_revoked",
		"NATS users whose public key is on their account's revocation list. Their signed credentials no longer connect.",
		nil, nil,
	)
	descDatabaseBytes = prometheus.NewDesc(
		"stone_age_database_size_bytes",
		"Size of the PocketBase SQLite database on disk, including its WAL. The number to alert on for disk growth.",
		nil, nil,
	)
	// A gauge, and named like one. It counts failures in THIS scrape, not
	// failures since start, so the _total suffix a counter would carry would be
	// a lie about what `rate()` on it means.
	descScrapeError = prometheus.NewDesc(
		"stone_age_collector_errors",
		"Collectors that failed during this scrape. Non-zero means some series below are missing, not that they are zero.",
		nil, nil,
	)
)

func (c *dbCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descRecords
	ch <- descInactive
	ch <- descRevokedUsers
	ch <- descDatabaseBytes
	ch <- descScrapeError
}

func (c *dbCollector) Collect(ch chan<- prometheus.Metric) {
	var failures float64

	// A collection that errors is SKIPPED rather than reported as zero. Zero is
	// a legitimate value here ("no leaf nodes configured"), so emitting it on
	// failure would turn a broken query into a confident wrong answer — and an
	// alert on `== 0` would fire for the wrong reason. The absent series plus
	// stone_age_collector_errors_total says what actually happened.
	count := func(collection string, desc *prometheus.Desc, labels []string, exprs ...dbx.Expression) {
		if collection == "" {
			return
		}
		n, err := c.app.CountRecords(collection, exprs...)
		if err != nil {
			failures++
			return
		}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(n), labels...)
	}

	for _, col := range []string{
		c.opts.OrgCollection,
		c.opts.MembershipCollection,
		"users",
		"things",
		"locations",
		"leaf_nodes",
		"thing_types",
		"location_types",
		"message_schemas",
		c.opts.NatsAccountCollection,
		c.opts.NatsUserCollection,
		c.opts.NebulaHostCollection,
		c.opts.AuditCollection,
	} {
		count(col, descRecords, []string{col})
	}

	// The decommissioning flag from CLAUDE.md's "A flag is not a control unless
	// something acts on it": these two have teeth (token kill + NATS revoke), so
	// they are worth counting.
	inactive := dbx.HashExp{"active": false}
	count("things", descInactive, []string{"things"}, inactive)
	count("leaf_nodes", descInactive, []string{"leaf_nodes"}, inactive)

	count(c.opts.NatsUserCollection, descRevokedUsers, nil, dbx.HashExp{"revoke": true})

	if size, ok := databaseBytes(c.app); ok {
		ch <- prometheus.MustNewConstMetric(descDatabaseBytes, prometheus.GaugeValue, float64(size))
	} else {
		failures++
	}

	ch <- prometheus.MustNewConstMetric(descScrapeError, prometheus.GaugeValue, failures)
}

// databaseBytes sums data.db and its write-ahead log. The WAL is included
// because it is real disk the operator has to have, and on a busy install it is
// routinely the larger of the two.
func databaseBytes(app core.App) (int64, bool) {
	base := filepath.Join(app.DataDir(), "data.db")
	var total int64
	var found bool
	for _, suffix := range []string{"", "-wal", "-shm"} {
		info, err := os.Stat(base + suffix)
		if err != nil {
			continue
		}
		total += info.Size()
		found = true
	}
	return total, found
}

// ---------------------------------------------------------- embedded NATS

// embeddedNATSCollector exports the in-process NATS server's own counters, and
// emits nothing at all when the bus is a separate process.
//
// Emitting nothing is the point. With an external nats-server these numbers are
// not this process's to report, and publishing zeros would look like a quiet
// bus rather than an absent one. Operators running an external server should
// scrape it with prometheus-nats-exporter, which reads the server's own
// monitoring port and reports far more than this ever could.
type embeddedNATSCollector struct {
	server func() *natsd.Server
}

var (
	descNATSUp = prometheus.NewDesc(
		"stone_age_nats_embedded_up",
		"1 when this process is running the NATS server in-process (serve --nats). Absent entirely with an external server.",
		nil, nil,
	)
	descNATSConns = prometheus.NewDesc(
		"stone_age_nats_connections",
		"Client connections to the embedded NATS server.",
		nil, nil,
	)
	descNATSRoutes = prometheus.NewDesc(
		"stone_age_nats_cluster_routes",
		"Peer connections to other servers in this NATS cluster. Zero on a single-node deployment, "+
			"which is the default; non-zero when a `cluster` block in nats.conf peers this Control Plane with other nodes.",
		nil, nil,
	)
	descNATSLeafs = prometheus.NewDesc(
		"stone_age_nats_leafnode_connections",
		"Leaf-node CONNECTIONS attached to the embedded NATS server. A TCP session the server can see — "+
			"not the same fact as a leaf-sync heartbeat, which says the edge agent is actually syncing.",
		nil, nil,
	)
	descNATSSlow = prometheus.NewDesc(
		"stone_age_nats_slow_consumers_total",
		"Slow consumers detected by the embedded NATS server since it started.",
		nil, nil,
	)
	descNATSMsgs = prometheus.NewDesc(
		"stone_age_nats_msgs_total",
		"Messages the embedded NATS server has handled, by direction.",
		[]string{"direction"}, nil,
	)
	descNATSBytes = prometheus.NewDesc(
		"stone_age_nats_bytes_total",
		"Bytes the embedded NATS server has handled, by direction.",
		[]string{"direction"}, nil,
	)
	descNATSJetStream = prometheus.NewDesc(
		"stone_age_nats_jetstream_bytes",
		"JetStream usage on the embedded NATS server, by storage tier.",
		[]string{"tier"}, nil,
	)
)

func (c *embeddedNATSCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descNATSUp
	ch <- descNATSConns
	ch <- descNATSRoutes
	ch <- descNATSLeafs
	ch <- descNATSSlow
	ch <- descNATSMsgs
	ch <- descNATSBytes
	ch <- descNATSJetStream
}

func (c *embeddedNATSCollector) Collect(ch chan<- prometheus.Metric) {
	stats, ok := c.server().Stats()
	if !ok {
		return // external bus, or the server has not started yet
	}
	g := func(d *prometheus.Desc, v float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, labels...)
	}
	cnt := func(d *prometheus.Desc, v float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v, labels...)
	}

	g(descNATSUp, 1)
	g(descNATSConns, float64(stats.Connections))
	g(descNATSRoutes, float64(stats.Routes))
	g(descNATSLeafs, float64(stats.Leafnodes))
	cnt(descNATSSlow, float64(stats.SlowConsumers))
	cnt(descNATSMsgs, float64(stats.InMsgs), "in")
	cnt(descNATSMsgs, float64(stats.OutMsgs), "out")
	cnt(descNATSBytes, float64(stats.InBytes), "in")
	cnt(descNATSBytes, float64(stats.OutBytes), "out")
	if stats.JetStreamEnabled {
		g(descNATSJetStream, float64(stats.JetStreamMemory), "memory")
		g(descNATSJetStream, float64(stats.JetStreamStore), "file")
	}
}

// ------------------------------------------------------------------- HTTP

// httpMetrics instruments the API surface.
//
// The route label is the matched PATTERN, never the request path. Paths here
// carry record ids (/api/collections/things/records/abc123def456789), so
// labelling by path would mint a new time series per record touched — the
// classic cardinality bomb, and on a platform whose whole job is holding
// per-device rows it would be one series per device per method.
type httpMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// registerHTTPMetrics instruments every request PocketBase serves; the
// middleware is bound to the router on OnServe.
func registerHTTPMetrics(app *pocketbase.PocketBase, set *metrics.Set) {
	m := &httpMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "stone_age_http_requests_total",
			Help: "HTTP requests served, by matched route pattern, method and status class.",
		}, []string{"route", "method", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "stone_age_http_request_duration_seconds",
			Help: "HTTP request latency by matched route pattern.",
			// Default buckets, which top out at 10s. Adequate here: anything
			// slower than that is a timeout story, not a latency story.
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method"}),
	}
	set.Registry.MustRegister(m.requests, m.duration)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.BindFunc(func(re *core.RequestEvent) error {
			started := time.Now()
			err := re.Next()

			route := routePattern(re)
			method := re.Request.Method

			m.duration.WithLabelValues(route, method).Observe(time.Since(started).Seconds())
			m.requests.WithLabelValues(route, method, statusClass(re, err)).Inc()
			return err
		})
		return se.Next()
	})
}

// routePattern returns the registered pattern for the matched route, falling
// back to "other" rather than to the raw path. "other" loses detail; the raw
// path would lose the server.
//
// Go's ServeMux patterns are "GET /api/health" — method and path in one string.
// The method is stripped here because it is already its own label, and leaving
// it in both places means every query has to know that route="GET /x" and
// method="GET" are the same fact stated twice.
func routePattern(re *core.RequestEvent) string {
	p := re.Request.Pattern
	if p == "" {
		return "other"
	}
	if i := strings.IndexByte(p, ' '); i >= 0 {
		return p[i+1:]
	}
	return p
}

// statusClass buckets the response into 2xx/4xx/5xx rather than exact codes.
//
// Exact codes would trip over this platform's own conventions more than they
// would help: PocketBase answers 404 when an update rule rejects and 400 on a
// denied create, so "how many 404s" is a question about authorization, traffic
// and genuinely missing records all at once. The class is what an alert wants;
// the audit log and the access log have the specifics.
func statusClass(re *core.RequestEvent, err error) string {
	status := re.Status()
	if status == 0 {
		// The response was never written — an error short-circuited the chain
		// before the status was set.
		if err != nil {
			return "5xx"
		}
		return "unknown"
	}
	return strconv.Itoa(status/100) + "xx"
}
