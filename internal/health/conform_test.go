package health

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stone-age-io/kit/health/healthtest"
)

// The platform keeps its own health engine; the kit's conformance test is what
// keeps it on the wire format every Stone-Age.io process shares (Health & Metrics §2,
// app-kit phase 4). The handler here is the body of the GET /api/ready route
// in hooks/observability.go, driven through every state worth covering.
func TestReadyConformsToTheSharedWireFormat(t *testing.T) {
	cases := []struct {
		name    string
		results map[string]Result
	}{
		{"ready", map[string]Result{"database": OK("ok"), "nats": OK("connected")}},
		{"ready with a warning", map[string]Result{"database": OK("ok"), "nebula_ca": Warn("expires soon", "rotate it")}},
		{"failing", map[string]Result{"database": OK("ok"), "nats": Fail("unreachable", "start NATS")}},
		{"skipped only", map[string]Result{"metrics": Skip("off")}},
		{"a panicking check", map[string]Result{"database": OK("ok")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reg Registry
			for name, res := range tc.results {
				reg.Register(name, fixed(res))
			}
			if tc.name == "a panicking check" {
				reg.Register("broken", func(context.Context) Result { panic("boom") })
			}
			prober := NewProber(&reg, "v1.2.3", time.Hour, time.Second)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				WriteJSON(w, prober.Snapshot())
			})

			if tc.name == "ready" {
				// Before the first probe: the answer the route gives while the
				// first probe is still dialling NATS.
				healthtest.Conform(t, handler)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			prober.Start(ctx)
			healthtest.Conform(t, handler)
		})
	}
}
