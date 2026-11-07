package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	"platform/internal/health"
)

func reportWith(t *testing.T, results map[string]health.Result) *health.Report {
	t.Helper()
	var reg health.Registry
	for name, res := range results {
		reg.Register(name, func(res health.Result) health.Func {
			return func(context.Context) health.Result { return res }
		}(res))
	}
	return reg.Run(context.Background(), "v0.2.0", time.Minute)
}

func scrape(t *testing.T, s *Set, token string, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler(token).ServeHTTP(rec, req)
	return rec
}

// The exposition must parse with Prometheus's own parser. Nothing in CI scrapes
// it, so a malformed body would otherwise look fine and be unscrapeable.
func TestExpositionParses(t *testing.T) {
	s := New("stone_age", "v0.2.0")
	s.Observe(reportWith(t, map[string]health.Result{
		"database": health.OK("responding"),
		// Quotes, backslashes and newlines appear in real check details and
		// would break the scrape if one leaked into a label.
		"nats_trust": health.Fail(`rejected "sys": C:\path\x`+"\nsecond line", "fix it"),
	}))

	rec := scrape(t, s, "", httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	// UTF8Validation is what a current Prometheus server uses; a zero-value
	// TextParser panics.
	parser := expfmt.NewTextParser(model.UTF8Validation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(rec.Body.String()))
	if err != nil {
		t.Fatalf("Prometheus could not parse our own exposition: %v", err)
	}
	for _, want := range []string{
		"stone_age_ready",
		"stone_age_check_state",
		"stone_age_check_timestamp_seconds",
		"stone_age_build_info",
		"go_goroutines", // the runtime collector is registered
	} {
		if _, ok := families[want]; !ok {
			t.Errorf("missing metric family %q", want)
		}
	}
}

// promlint catches naming mistakes such as a counter without _total or a
// missing help string.
func TestExpositionPassesPromlint(t *testing.T) {
	s := New("stone_age", "v0.2.0")
	s.Observe(reportWith(t, map[string]health.Result{"database": health.OK("responding")}))

	problems, err := testutil.CollectAndLint(s.Registry)
	if err != nil {
		t.Fatalf("lint failed: %v", err)
	}
	for _, p := range problems {
		// Only our own namespace; the upstream collectors have known quirks.
		if strings.HasPrefix(p.Metric, "stone_age_") {
			t.Errorf("promlint: %s: %s", p.Metric, p.Text)
		}
	}
}

// A check's state is a complete state set: one series per state, exactly one of
// them 1. A boolean would collapse warn and skipped into "not ok" (see
// TestSkippedRanksBelowOK).
func TestCheckStateIsACompleteSet(t *testing.T) {
	s := New("stone_age", "v0.2.0")
	s.Observe(reportWith(t, map[string]health.Result{
		"a": health.OK(""),
		"b": health.Warn("", ""),
		"c": health.Fail("", ""),
		"d": health.Skip(""),
	}))

	want := map[string]health.State{
		"a": health.StateOK,
		"b": health.StateWarn,
		"c": health.StateFail,
		"d": health.StateSkipped,
	}

	for name, active := range want {
		var hot int
		for _, st := range allStates {
			got := testutil.ToFloat64(s.checkState.WithLabelValues(name, string(st)))
			switch {
			case st == active && got != 1:
				t.Errorf("check %q state %q = %v, want 1", name, st, got)
			case st != active && got != 0:
				t.Errorf("check %q state %q = %v, want 0", name, st, got)
			}
			if got == 1 {
				hot++
			}
		}
		if hot != 1 {
			t.Errorf("check %q has %d states set to 1, want exactly 1", name, hot)
		}
	}
}

// Warnings do not clear stone_age_ready, matching the HTTP status. An alert
// wired to this gauge must not fire because at-rest encryption is off.
func TestReadyGaugeTracksReadinessNotWarnings(t *testing.T) {
	s := New("stone_age", "v0.2.0")

	s.Observe(reportWith(t, map[string]health.Result{"a": health.Warn("", "")}))
	if got := testutil.ToFloat64(s.ready); got != 1 {
		t.Errorf("ready = %v with only a warning, want 1", got)
	}

	s.Observe(reportWith(t, map[string]health.Result{"a": health.Fail("", "")}))
	if got := testutil.ToFloat64(s.ready); got != 0 {
		t.Errorf("ready = %v with a failure, want 0", got)
	}
}

// No report yet must read as not ready, as the HTTP endpoint's nil handling does.
func TestObserveNilIsNotReady(t *testing.T) {
	s := New("stone_age", "v0.2.0")
	s.Observe(nil)
	if got := testutil.ToFloat64(s.ready); got != 0 {
		t.Errorf("ready = %v before any probe, want 0", got)
	}
}

// Two Sets must not collide: each has a private registry, because
// slackhq/nebula registers collectors into the default one.
func TestSetsUsePrivateRegistries(t *testing.T) {
	a := New("stone_age", "v1")
	b := New("agent", "v1")
	if a.Registry == b.Registry {
		t.Fatal("two Sets share a registry")
	}
	// A shared registry would have panicked in New.
	a.Observe(reportWith(t, map[string]health.Result{"x": health.OK("")}))
	b.Observe(reportWith(t, map[string]health.Result{"x": health.OK("")}))
}

// The default is an open endpoint, by design (see Handler).
func TestNoTokenMeansOpen(t *testing.T) {
	s := New("stone_age", "v1")
	rec := scrape(t, s, "", httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d with no token configured, want 200", rec.Code)
	}
}

// With a token set, both Bearer and Basic must work, or an existing scrape
// config using the other form gets 401.
func TestTokenAcceptsBearerAndBasic(t *testing.T) {
	const token = "s3cret-scrape-token"
	s := New("stone_age", "v1")

	bearer := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	bearer.Header.Set("Authorization", "Bearer "+token)
	if rec := scrape(t, s, token, bearer); rec.Code != http.StatusOK {
		t.Errorf("Bearer: status = %d, want 200", rec.Code)
	}

	// Any username; the token is the password.
	basic := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	basic.SetBasicAuth("prometheus", token)
	if rec := scrape(t, s, token, basic); rec.Code != http.StatusOK {
		t.Errorf("Basic: status = %d, want 200", rec.Code)
	}
}

func TestTokenRejectsWrongAndMissing(t *testing.T) {
	const token = "s3cret-scrape-token"
	s := New("stone_age", "v1")

	cases := map[string]func(*http.Request){
		"no credentials":  func(*http.Request) {},
		"wrong bearer":    func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") },
		"wrong basic":     func(r *http.Request) { r.SetBasicAuth("prometheus", "nope") },
		"token as user":   func(r *http.Request) { r.SetBasicAuth(token, "") },
		"bearer no space": func(r *http.Request) { r.Header.Set("Authorization", "Bearer"+token) },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			mutate(req)
			rec := scrape(t, s, token, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if rec.Body.Len() > 0 && strings.Contains(rec.Body.String(), "stone_age_") {
				t.Error("a rejected scrape leaked metrics in the body")
			}
		})
	}
}
