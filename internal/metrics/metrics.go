// Package metrics is the Prometheus exposition shared by the Control Plane and
// the edge agent.
//
// Prometheus because integrators already run tools that read it. The client
// library rather than a hand-written encoder, because nothing in CI scrapes the
// output to catch a malformed exposition; slackhq/nebula already links it in.
//
// No per-organization labels. The Control Plane can only count its own rows, so
// per-org series would be customer-identifying, of no operational use, on an
// endpoint open by default. Per-site numbers live inside an organization's NATS
// account, which this process has no credential for; the agent exposes them.
package metrics

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"platform/internal/health"
)

// allStates is every state a check can report. Each gets a series, so a warned
// check stays distinct from one never run.
var allStates = []health.State{
	health.StateOK,
	health.StateWarn,
	health.StateFail,
	health.StateSkipped,
}

// Set is one process's metrics: its own registry plus the readiness gauges
// every binary here publishes.
//
// A private registry, because slackhq/nebula registers its own collectors into
// the default one when a Nebula host runs in-process.
type Set struct {
	Registry *prometheus.Registry

	ready      prometheus.Gauge
	checkState *prometheus.GaugeVec
	checkTime  prometheus.Gauge
}

// New builds a metrics set under the given namespace ("stone_age", "agent")
// and registers the Go runtime and process collectors alongside it.
func New(namespace, version string) *Set {
	reg := prometheus.NewRegistry()

	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	// The conventional build-info pattern: a constant 1 whose labels carry the
	// facts.
	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "build_info",
		Help:      "Build information. Always 1; read the labels.",
	}, []string{"version"})
	buildInfo.WithLabelValues(version).Set(1)
	reg.MustRegister(buildInfo)

	s := &Set{
		Registry: reg,
		ready: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ready",
			Help:      "1 if the last readiness probe passed, 0 if any check failed. Warnings do not clear this.",
		}),
		checkState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "check_state",
			Help:      "Readiness check state set: 1 on the check's current state, 0 on the others.",
		}, []string{"name", "state"}),
		checkTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "check_timestamp_seconds",
			Help:      "Unix time of the last readiness probe. Alert on this going stale — it means the prober itself is wedged.",
		}),
	}
	reg.MustRegister(s.ready, s.checkState, s.checkTime)
	return s
}

// Observe copies a readiness report into the gauges. A nil report sets ready to
// 0, because "never probed" should read as not ready.
func (s *Set) Observe(rep *health.Report) {
	if rep == nil {
		s.ready.Set(0)
		return
	}
	if rep.Ready {
		s.ready.Set(1)
	} else {
		s.ready.Set(0)
	}
	s.checkTime.Set(float64(rep.Checked.Unix()))

	for _, c := range rep.Checks {
		for _, st := range allStates {
			v := 0.0
			if c.State == st {
				v = 1.0
			}
			s.checkState.WithLabelValues(c.Name, string(st)).Set(v)
		}
	}
}

// Bind wires a prober's reports into this set, updating the gauges after every
// probe. Call it before Prober.Start.
//
// Updating from the prober rather than at scrape time keeps NATS dials off the
// scrape path: a slow check shows as a stale check_timestamp_seconds instead of
// a timed-out scrape.
func (s *Set) Bind(p *health.Prober) {
	p.OnRefresh(s.Observe)
}

// Handler returns the /metrics handler, optionally protected by a shared token.
//
// An empty token, the default, means no authentication: the series are
// aggregate counts with no customer identifiers, and an endpoint that needs a
// credential mostly does not get scraped.
//
// The token is accepted as `Authorization: Bearer <token>` or as the HTTP Basic
// password with any username, since scrapers support one or the other.
// PocketBase auth is not used: its JWTs expire, and scrapers cannot refresh.
func (s *Set) Handler(token string) http.Handler {
	h := promhttp.HandlerFor(s.Registry, promhttp.HandlerOpts{
		// A broken collector should not take the endpoint down.
		ErrorHandling: promhttp.ContinueOnError,
	})
	if token == "" {
		return h
	}
	return requireToken(token, h)
}

func requireToken(token string, next http.Handler) http.Handler {
	want := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tokenMatches(r, want) {
			next.ServeHTTP(w, r)
			return
		}
		// Advertise Basic so a browser offers a prompt.
		w.Header().Set("WWW-Authenticate", `Basic realm="metrics"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

func tokenMatches(r *http.Request, want []byte) bool {
	if _, pass, ok := r.BasicAuth(); ok {
		if subtle.ConstantTimeCompare([]byte(pass), want) == 1 {
			return true
		}
	}
	if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		if subtle.ConstantTimeCompare([]byte(bearer), want) == 1 {
			return true
		}
	}
	return false
}
