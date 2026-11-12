package hooks

import (
	"context"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"platform/internal/health"
	"platform/internal/metrics"
)

// ObservabilityOptions is everything the readiness checks and the metrics
// collectors read: one struct, so the call site names each collection once.
type ObservabilityOptions struct {
	Version string

	OrgCollection         string
	MembershipCollection  string
	NatsAccountCollection string
	NatsUserCollection    string
	NebulaHostCollection  string
	NebulaCACollection    string
	AuditCollection       string

	// NatsServerURL is nats.server_url: the TCP address THIS process dials,
	// checked for reachability and operator trust.
	NatsServerURL string
	// NatsWebsocketURLs is nats.websocket_urls: what a browser dials. Only
	// checked for being configured; this process often cannot reach it.
	NatsWebsocketURLs []string

	NatsEncryptionKey   string
	NebulaEncryptionKey string

	ProbeInterval time.Duration
	ProbeTimeout  time.Duration

	MetricsEnabled bool
	MetricsToken   string
}

// RegisterObservability wires up GET /api/ready and GET /metrics.
//
// /api/ready is separate from PocketBase's /api/health, which answers liveness.
// Failing liveness restarts the process; failing readiness stops traffic to
// it. One endpoint for both would restart-loop every time NATS blipped.
//
// It is unauthenticated and serves one public body: its callers
// (orchestrators, load balancers, uptime checkers) hold no session, and
// /metrics is open by default and already publishes every check's state. An
// operator who wants either closed puts it behind their proxy.
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

		// No switch for /api/ready, deliberately: it is a contract with the
		// orchestrator. /metrics is only a reporting channel.
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
		// Off the startup path: the first probe dials NATS, which would delay
		// the listener when NATS is down. Meanwhile /api/ready answers "503, not
		// probed yet".
		go prober.Start(ctx)
		return se.Next()
	})
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		stop()
		return e.Next()
	})
}
