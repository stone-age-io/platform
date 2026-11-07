package health

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// Default probe timings, deliberately slower than an orchestrator polls: the
// checks hit SQLite and dial NATS.
const (
	DefaultProbeInterval = 15 * time.Second
	DefaultProbeTimeout  = 5 * time.Second
)

// Prober runs a Registry on an interval and caches the latest Report.
//
// Cached, not run per request: a readiness endpoint is polled by things that
// restart containers, and a slow check inside the request would blow the
// probe's timeout and kill a healthy container. A hung check shows as a stale
// report instead. A report is at most one interval old; Report.Checked says
// when.
type Prober struct {
	reg      *Registry
	version  string
	interval time.Duration
	timeout  time.Duration
	started  time.Time
	snap     atomic.Pointer[Report]

	// onRefresh fires after every probe, so metrics never look like a wedged
	// prober. onChange fires only when Ready flips, so logs carry transitions.
	onRefresh func(*Report)
	onChange  func(*Report)
}

// NewProber builds a prober. A zero interval or timeout uses the defaults.
func NewProber(reg *Registry, version string, interval, timeout time.Duration) *Prober {
	if interval <= 0 {
		interval = DefaultProbeInterval
	}
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	return &Prober{
		reg:      reg,
		version:  version,
		interval: interval,
		timeout:  timeout,
		started:  time.Now(),
	}
}

// OnRefresh registers a callback fired after every probe. Call it before Start.
func (p *Prober) OnRefresh(fn func(*Report)) { p.onRefresh = fn }

// OnChange registers a callback fired when the ready flag flips. Call it before
// Start.
func (p *Prober) OnChange(fn func(*Report)) { p.onChange = fn }

// Start runs the checks once synchronously, then keeps them refreshed in the
// background until ctx is cancelled. The first run is synchronous so the first
// probe never reads a nil snapshot, which would look like "not ready".
func (p *Prober) Start(ctx context.Context) {
	p.refresh(ctx)
	go func() {
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.refresh(ctx)
			}
		}
	}()
}

func (p *Prober) refresh(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	rep := p.reg.Run(runCtx, p.version, time.Since(p.started))
	prev := p.snap.Swap(rep)

	if p.onRefresh != nil {
		p.onRefresh(rep)
	}
	if p.onChange != nil && (prev == nil || prev.Ready != rep.Ready) {
		p.onChange(rep)
	}
}

// Snapshot returns the most recent Report, or nil before the first run.
func (p *Prober) Snapshot() *Report { return p.snap.Load() }

// statusCode is the HTTP status a readiness response should carry: 200 when
// ready, 503 when not. Warnings return 200: a restart fixes no warning, it only
// takes a working deployment offline.
func statusCode(rep *Report) int {
	if rep == nil || !rep.Ready {
		return http.StatusServiceUnavailable
	}
	return http.StatusOK
}

// WriteJSON writes a Report as an HTTP response with the right status code.
// A nil report (not probed yet) is a 503 with a body saying so.
func WriteJSON(w http.ResponseWriter, rep *Report) {
	w.Header().Set("Content-Type", "application/json")
	// A cached readiness answer is worse than none.
	w.Header().Set("Cache-Control", "no-store")

	if rep == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ready":  false,
			"state":  StateFail,
			"checks": []Result{},
			"detail": "readiness has not been probed yet",
		})
		return
	}

	w.WriteHeader(statusCode(rep))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ") // read by humans with curl at least as often as by probes
	_ = enc.Encode(rep)
}

// Handler serves the cached report over plain net/http, for the agent. The
// Control Plane calls WriteJSON from its own PocketBase route.
func (p *Prober) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, p.Snapshot())
	})
}

// LogReport prints a readiness transition. Used as the OnChange callback by
// both binaries so the two logs read the same.
func LogReport(prefix string, rep *Report) {
	if rep.Ready {
		if rep.State == StateWarn {
			log.Printf("✅ %s: ready (with warnings)", prefix)
			for _, c := range rep.Checks {
				if c.State == StateWarn {
					log.Printf("   ⚠️  %s: %s", c.Name, c.Detail)
				}
			}
			return
		}
		log.Printf("✅ %s: ready", prefix)
		return
	}
	log.Printf("❌ %s: NOT ready", prefix)
	for _, c := range rep.Checks {
		if c.State != StateFail {
			continue
		}
		log.Printf("   ❌ %s: %s", c.Name, c.Detail)
		if c.Fix != "" {
			log.Printf("      → %s", c.Fix)
		}
	}
}
