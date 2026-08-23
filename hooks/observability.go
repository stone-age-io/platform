package hooks

import (
	"context"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"platform/internal/health"
	"platform/internal/metrics"
	"platform/internal/natsd"
)

// ObservabilityOptions is everything the readiness checks and the metrics
// collectors read. One struct rather than one per file: these are all
// deployment facts main.go already has in hand, and splitting them into a
// ReadinessOptions and a MetricsOptions meant naming the same collections twice
// at the call site with no reader ever able to tell why.
type ObservabilityOptions struct {
	Version string

	OrgCollection         string
	MembershipCollection  string
	NatsAccountCollection string
	NatsUserCollection    string
	NebulaHostCollection  string
	AuditCollection       string

	// NatsServerURL is nats.server_url: the TCP address THIS process dials.
	// Checked for reachability, and for whether the server there trusts this
	// platform's operator.
	NatsServerURL string
	// NatsWebsocketURLs is nats.websocket_urls: what a browser dials. Not
	// reachability-checked, because this process is not a browser and often
	// cannot reach the address at all — only checked for being configured.
	NatsWebsocketURLs []string
	// EmbeddedNATS returns the in-process NATS server, or nil when the bus is a
	// separate process. A func rather than a value because `serve --nats`
	// starts the server during OnServe, after this registration runs.
	EmbeddedNATS func() *natsd.Server

	NatsEncryptionKey   string
	NebulaEncryptionKey string

	ProbeInterval time.Duration
	ProbeTimeout  time.Duration

	MetricsEnabled bool
	MetricsToken   string
}

// RegisterObservability wires up GET /api/ready and GET /metrics.
//
// WHY /api/ready RATHER THAN EXTENDING /api/health. PocketBase's own
// /api/health is a liveness answer — the process is up and serving — and it is
// correct as it stands. Readiness is a different question with a different
// consequence: liveness failing should restart the process, readiness failing
// should stop sending it traffic. Overloading one endpoint with both would mean
// a container restart loop every time NATS was briefly unreachable, which fixes
// nothing and takes the console down with it.
//
// WHY IT IS UNAUTHENTICATED, AND WHY THE WHOLE BODY IS PUBLIC. The callers are
// container orchestrators, load balancers and uptime checkers, none of which
// hold a PocketBase session. An earlier version withheld each check's detail
// and suggested fix from anonymous callers on the grounds that they name
// internal hostnames and whether at-rest encryption is off — but /metrics is
// open by default and already publishes every check's state, so the withholding
// bought a partial boundary at the cost of a second response shape to maintain
// and reason about. Serve one body. An operator who wants this closed puts it
// behind their proxy, which is the same lever that closes /metrics.
func RegisterObservability(app *pocketbase.PocketBase, opts ObservabilityOptions) {
	set := metrics.New("stone_age", opts.Version)
	registerHTTPMetrics(app, set)
	registerPlatformMetrics(app, set, opts)

	var checks health.Registry
	registerPlatformChecks(app, &checks, opts)

	prober := health.NewProber(&checks, opts.Version, opts.ProbeInterval, opts.ProbeTimeout)
	set.Bind(prober)
	prober.OnChange(func(rep *health.Report) { health.LogReport("readiness", rep) })

	metricsHandler := set.Handler(opts.MetricsToken)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/ready", func(re *core.RequestEvent) error {
			health.WriteJSON(re.Response, prober.Snapshot())
			return nil
		})

		// No switch for /api/ready, deliberately: a probe endpoint that can be
		// disabled is one some deployment will disable and then be unable to
		// explain. /metrics is different — it is a reporting channel, not a
		// contract with the orchestrator.
		if opts.MetricsEnabled {
			se.Router.GET("/metrics", func(re *core.RequestEvent) error {
				metricsHandler.ServeHTTP(re.Response, re.Request)
				return nil
			})
		}
		return se.Next()
	})

	// Probing starts with the listener and stops with the process.
	ctx, stop := context.WithCancel(context.Background())
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// Off the startup path deliberately. The first probe dials NATS, and
		// blocking here would delay the listener by the dial timeout in exactly
		// the case where NATS is down — while /api/ready already answers "503,
		// not probed yet", which is the correct thing for a readiness endpoint
		// to say during startup.
		go prober.Start(ctx)
		return se.Next()
	})
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		stop()
		return e.Next()
	})
}
