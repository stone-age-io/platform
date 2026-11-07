package hooks

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/router"
	"github.com/prometheus/client_golang/prometheus"

	"platform/internal/metrics"
	"platform/internal/natsd"
)

// registerPlatformMetrics adds the Control Plane's own collectors to a set:
// its own HTTP traffic, row counts in its own database, and, with `--nats`, the
// embedded NATS server's counters. That is all it can observe first-hand.
//
// `stone_age_records{collection="things"}` counts devices CONFIGURED, not
// online, so an alert on it can never fire. Liveness lives inside each
// organization's NATS account, which this process holds no credential for; a
// console widget asks $SYS.REQ.ACCOUNT.PING.CONNZ with the user's own
// credential, and the agent exports its own on the edge.
func registerPlatformMetrics(app core.App, set *metrics.Set, opts ObservabilityOptions) {
	set.Registry.MustRegister(&dbCollector{app: app, opts: opts})
	set.Registry.MustRegister(&certCollector{app: app, opts: opts})
	if opts.EmbeddedNATS != nil {
		set.Registry.MustRegister(&embeddedNATSCollector{server: opts.EmbeddedNATS})
	}
}

// --------------------------------------------------------------- database

// dbCollector counts rows at scrape time, unlike the check gauges, which use the
// prober's tick: these are local COUNT queries with no network in them, while
// the readiness checks dial NATS, which must not sit on the scrape path.
type dbCollector struct {
	app  core.App
	opts ObservabilityOptions
}

var (
	descRecords = prometheus.NewDesc(
		"stone_age_records",
		"Rows in a platform collection. This is inventory CONFIGURED, not anything online: "+
			"device liveness lives inside an organization NATS account, which this process cannot read.",
		[]string{"collection"}, nil,
	)
	descInactive = prometheus.NewDesc(
		"stone_age_inactive_records",
		"Rows with active = false: devices that have been decommissioned.",
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
	// A gauge of failures in THIS scrape, so no _total suffix.
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

	// A collection that errors is SKIPPED, not reported as zero, because zero is
	// a legitimate value. The absent series plus stone_age_collector_errors says
	// what happened.
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
		"thing_types",
		"location_types",
		// Counted rather than given a retention job: the feed grows with human
		// clicks, so a graph answers "does this need pruning" better than a cron.
		ActivityCollection,
		c.opts.NatsAccountCollection,
		c.opts.NatsUserCollection,
		c.opts.NebulaHostCollection,
		c.opts.AuditCollection,
	} {
		count(col, descRecords, []string{col})
	}

	// The decommissioning flag has teeth (token kill + NATS revoke), so it is
	// worth counting.
	count("things", descInactive, []string{"things"}, dbx.HashExp{"active": false})

	count(c.opts.NatsUserCollection, descRevokedUsers, nil, dbx.HashExp{"revoke": true})

	if size, ok := databaseBytes(c.app); ok {
		ch <- prometheus.MustNewConstMetric(descDatabaseBytes, prometheus.GaugeValue, float64(size))
	} else {
		failures++
	}

	ch <- prometheus.MustNewConstMetric(descScrapeError, prometheus.GaugeValue, failures)
}

// databaseBytes sums data.db, its WAL and its -shm file. The WAL is real disk
// and on a busy install often larger than the database.
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

// ------------------------------------------------------ certificate expiry

// certCollector publishes when the Nebula certificates this platform signed
// run out.
//
// The gauge is the absolute expiry in Unix seconds (the node_exporter
// convention), not a countdown, which is stale the moment it is stored. Alert
// with the horizon in the query: `stone_age_certificate_expiry_seconds - time()
// < 30 * 86400`.
//
// One series per kind, the soonest expiry, not one per certificate: a series
// per host would need an identifying label, which would publish a per-tenant
// device inventory on an endpoint that is open by default.
type certCollector struct {
	app  core.App
	opts ObservabilityOptions
}

var (
	descCertExpiry = prometheus.NewDesc(
		"stone_age_certificate_expiry_seconds",
		"Unix timestamp at which the SOONEST-expiring Nebula certificate of this kind expires. "+
			"Alert relative to time(), e.g. expiry - time() < 30*86400. Absent when no certificates of the kind exist.",
		[]string{"kind"}, nil,
	)
	descCertExpired = prometheus.NewDesc(
		"stone_age_certificates_expired",
		"Nebula certificates of this kind whose expiry has passed. A host certificate here is a device that "+
			"cannot join the overlay; a CA here invalidates every certificate under it.",
		[]string{"kind"}, nil,
	)
	descCertExpiring = prometheus.NewDesc(
		"stone_age_certificates_expiring",
		"Nebula certificates of this kind expiring within the console's warning window, not counting ones already expired.",
		[]string{"kind"}, nil,
	)
	descCertTotal = prometheus.NewDesc(
		"stone_age_certificates",
		"Nebula certificates of this kind that are in service. Host certificates count only active = true rows, "+
			"since a decommissioned device's lapsed certificate is not a fault.",
		[]string{"kind"}, nil,
	)
)

func (c *certCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descCertExpiry
	ch <- descCertExpired
	ch <- descCertExpiring
	ch <- descCertTotal
}

func (c *certCollector) Collect(ch chan<- prometheus.Metric) {
	sums, err := nebulaCertSummaries(c.app, c.opts, time.Now())
	if err != nil {
		// Emit nothing, as dbCollector does: zeros would read as "nothing is
		// expiring" when the truth is "we could not look".
		return
	}
	g := func(d *prometheus.Desc, v float64, kind string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, kind)
	}
	for _, s := range sums {
		g(descCertTotal, float64(s.Total), s.Kind)
		g(descCertExpired, float64(s.Expired), s.Kind)
		g(descCertExpiring, float64(s.Expiring), s.Kind)
		// Only when there is one: a zero timestamp is 1970, which every
		// "expires soon" alert would fire on forever.
		if !s.Earliest.IsZero() {
			g(descCertExpiry, float64(s.Earliest.Unix()), s.Kind)
		}
	}
}

// ---------------------------------------------------------- embedded NATS

// embeddedNATSCollector exports the in-process NATS server's own counters, and
// emits nothing when the bus is a separate process: zeros would look like a
// quiet bus rather than an absent one. Scrape an external server with
// prometheus-nats-exporter.
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
			"across every account, so it is not a per-tenant availability signal. A tenant asks its own "+
			"account via $SYS.REQ.ACCOUNT.PING.CONNZ.",
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

// httpMetrics instruments the API surface. The route label is the matched
// PATTERN, never the request path: paths carry record ids, so a path label
// would mint a series per record touched.
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
			// Default buckets top out at 10s; anything slower is a timeout.
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method"}),
	}
	set.Registry.MustRegister(m.requests, m.duration)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.BindFunc(func(re *core.RequestEvent) error {
			started := time.Now()
			err := re.Next()

			route := routePattern(re)
			method := methodLabel(re.Request.Method)

			m.duration.WithLabelValues(route, method).Observe(time.Since(started).Seconds())
			m.requests.WithLabelValues(route, method, statusClass(re, err)).Inc()
			return err
		})
		return se.Next()
	})
}

// routePattern returns the registered pattern for the matched route, falling
// back to "other", never the raw path. A ServeMux pattern is "GET /api/health";
// the method is stripped because it is already its own label.
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

// methodLabel keeps the method label to the standard verbs. The method is
// client input: Go accepts any token, and the catch-all route runs this
// middleware, so an unauthenticated client could mint a series per request.
func methodLabel(m string) string {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return m
	}
	return "other"
}

// statusClass buckets the response into 2xx/4xx/5xx rather than exact codes:
// PocketBase answers 404 when an update rule rejects and 400 on a denied
// create, so exact codes mix authorization with missing records.
//
// A status of 0 means the response was never written: a handler returned an
// error and the router's ErrorHandler runs AFTER the middleware unwinds. Every
// API-rule rejection arrives that way, so it must not be counted as 5xx.
// router.ToApiError is what ErrorHandler calls before WriteHeader
// (tools/router/router.go), so this reports the status the client got, as
// PocketBase's own request logging does (apis/middlewares.go).
func statusClass(re *core.RequestEvent, err error) string {
	return statusClassFor(re.Status(), err)
}

// statusClassFor is the whole decision, split out so it can be tested without
// constructing a core.RequestEvent.
func statusClassFor(status int, err error) string {
	if status == 0 {
		if err == nil {
			return "unknown"
		}
		status = router.ToApiError(err).Status
	}
	return strconv.Itoa(status/100) + "xx"
}
