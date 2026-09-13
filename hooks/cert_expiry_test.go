package hooks_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/prometheus/client_golang/prometheus/testutil/promlint"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	"platform/hooks"
	"platform/internal/health"
	"platform/internal/metrics"
)

// Nebula certificate expiry: the readiness check, the metric, and the one thing
// that must hold between them -- both are computed from the same scan, so a
// green check can never sit beside a metric saying something expired.
//
// The dates are written straight onto records rather than produced by signing
// real certificates with a chosen validity: validity_years is an integer, so
// "expires in 12 days" and "expired last week" are not states the signing path
// can be asked for at all. What is under test is the reading of expires_at,
// which is the column every consumer -- this check, the metric, and the
// console's ExpiryBadge -- actually looks at.

type certFixture struct {
	app  core.App
	opts hooks.ObservabilityOptions
	now  time.Time
}

// newCertFixture replaces the contents of both Nebula collections with one row
// per offset, each expiring that far from now. A zero offset leaves expires_at
// unset.
//
// It reuses the package's shared app (see managed_org_exports_test.go's
// TestMain) because standing one up costs ~20s, and clears the two collections
// first: organization provisioning has already minted a real CA per org with a
// multi-year expiry, which would drown any fixture.
func newCertFixture(t *testing.T, caOffsets, hostOffsets []time.Duration) certFixture {
	t.Helper()

	app := managed.app
	now := time.Now()
	f := certFixture{
		app: app,
		now: now,
		opts: hooks.ObservabilityOptions{
			OrgCollection:         "organizations",
			MembershipCollection:  "memberships",
			NatsAccountCollection: "nats_accounts",
			NatsUserCollection:    "nats_users",
			NebulaHostCollection:  "nebula_hosts",
			NebulaCACollection:    "nebula_ca",
			AuditCollection:       "audit_logs",
		},
	}

	// Hosts, then networks, then CAs: raw SQL does not enforce the relations,
	// but leaving a network behind pointing at a deleted CA would make the next
	// subtest's fixture fail for a reason that has nothing to do with expiry.
	for _, c := range []string{"nebula_hosts", "nebula_networks", "nebula_ca"} {
		if _, err := app.DB().NewQuery("DELETE FROM {{" + c + "}}").Execute(); err != nil {
			t.Fatalf("clear %s: %v", c, err)
		}
	}

	mk := func(collection string, i int, extra map[string]any) *core.Record {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatalf("%s: %v", collection, err)
		}
		rec := core.NewRecord(col)
		rec.Set("organization", managed.tenant.Id)
		for k, v := range extra {
			rec.Set(k, v)
		}
		if err := app.Save(rec); err != nil {
			t.Fatalf("save %s[%d]: %v", collection, i, err)
		}
		return rec
	}

	var firstCA *core.Record
	for i, d := range caOffsets {
		rec := mk("nebula_ca", i, map[string]any{
			"name":           fmt.Sprintf("fixture-ca-%d", i),
			"validity_years": 5,
		})
		setExpiry(t, app, "nebula_ca", rec.Id, now, d)
		if firstCA == nil {
			firstCA = rec
		}
	}

	// nebula_hosts.network_id is required, and a network needs a CA. Only built
	// when there are hosts to put in it.
	if len(hostOffsets) > 0 {
		if firstCA == nil {
			// Only here so the hosts have a network to belong to; left with the
			// healthy expiry pb-nebula signs it with, so it never influences a
			// case whose caOffsets said nothing about a CA.
			firstCA = mk("nebula_ca", 0, map[string]any{
				"name":           "fixture-ca-implied",
				"validity_years": 5,
			})
		}
		network := mk("nebula_networks", 0, map[string]any{
			"name":       "fixture-net",
			"cidr_range": "10.10.0.0/16",
			"active":     true,
			"ca_id":      firstCA.Id,
		})

		for i, d := range hostOffsets {
			// nebula_hosts is a PocketBase AUTH collection, so email and
			// password are mandatory and email must be unique.
			rec := mk("nebula_hosts", i, map[string]any{
				"hostname":        fmt.Sprintf("fixture-host-%d", i),
				"email":           fmt.Sprintf("host-%d@fixture.test", i),
				"emailVisibility": true,
				"password":        "fixture-password-123",
				"overlay_ip":      fmt.Sprintf("10.10.0.%d", i+10),
				"network_id":      network.Id,
				"validity_years":  1,
				"active":          true,
			})
			setExpiry(t, app, "nebula_hosts", rec.Id, now, d)
		}
	}
	return f
}

// setExpiry writes expires_at directly, and has to.
//
// pb-nebula signs the certificate in its own create hook and derives expires_at
// from validity_years, so a Set() on the record before Save is overwritten:
// every host came back expiring in exactly one year no matter what the fixture
// asked for, and the first version of these tests "passed" a check that was
// reporting everything healthy. validity_years is an integer anyway, so
// "expired last week" is not a state the signing path can be asked for.
//
// A zero offset clears the column, which is the no-readable-expiry case.
func setExpiry(t *testing.T, app core.App, collection, id string, now time.Time, offset time.Duration) {
	t.Helper()

	value := ""
	if offset != 0 {
		at, err := types.ParseDateTime(now.Add(offset))
		if err != nil {
			t.Fatalf("parse offset: %v", err)
		}
		value = at.String()
	}

	_, err := app.DB().
		NewQuery("UPDATE {{" + collection + "}} SET expires_at = {:v} WHERE id = {:id}").
		Bind(dbx.Params{"v": value, "id": id}).
		Execute()
	if err != nil {
		t.Fatalf("set %s.expires_at: %v", collection, err)
	}
}

func runCertCheck(t *testing.T, f certFixture) health.Result {
	t.Helper()
	var reg health.Registry
	hooks.RegisterPlatformChecks(f.app, &reg, f.opts)
	rep := reg.Run(context.Background(), "test", time.Minute)
	for _, r := range rep.Checks {
		if r.Name == hooks.CertCheckName {
			return r
		}
	}
	t.Fatalf("no %q check was registered", hooks.CertCheckName)
	return health.Result{}
}

func grepLines(body, prefix string) string {
	var out []string
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, prefix) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func TestCertExpiryCountsExpiredExpiringAndValid(t *testing.T) {
	day := 24 * time.Hour
	f := newCertFixture(t,
		[]time.Duration{400 * day},                     // CA: healthy
		[]time.Duration{-3 * day, 10 * day, 200 * day}, // hosts: expired, expiring, healthy
	)

	sums, err := hooks.NebulaCertSummaries(f.app, f.opts, f.now)
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if len(sums) != 2 {
		t.Fatalf("want 2 summaries (CA then host), got %d", len(sums))
	}

	ca, host := sums[0], sums[1]
	if ca.Kind != hooks.CertKindCA || host.Kind != hooks.CertKindHost {
		t.Fatalf("summaries are out of order: %q then %q -- the metric's label values depend on this", ca.Kind, host.Kind)
	}
	if ca.Total != 1 || ca.Expired != 0 || ca.Expiring != 0 {
		t.Errorf("CA: total=%d expired=%d expiring=%d, want 1/0/0", ca.Total, ca.Expired, ca.Expiring)
	}
	if host.Total != 3 || host.Expired != 1 || host.Expiring != 1 {
		t.Errorf("hosts: total=%d expired=%d expiring=%d, want 3/1/1", host.Total, host.Expired, host.Expiring)
	}

	// Expired and Expiring must not double-count: an expired certificate is not
	// also "expiring soon", or every alert on the second series keeps firing
	// forever once the first one does.
	if host.Total-host.Expired-host.Expiring != 1 {
		t.Errorf("the three counts do not partition: %d - %d - %d", host.Total, host.Expired, host.Expiring)
	}
}

// TestCAGetsALongerWarningWindowThanAHost pins the one thing that stops the two
// windows collapsing back into a shared constant.
//
// A host certificate is renewable, and pb-nebula re-issues one automatically
// once it has burned through its lifetime -- so 30 days is ample. A CA cannot be
// renewed at all; the only remedy is rotation, which needs a wait in the middle
// long enough for every host to fetch a config nobody told it to fetch. Nebula's
// guide asks for two to three months.
//
// The certificate used here sits BETWEEN the two windows on purpose. With one
// shared 30-day constant it reads as healthy, and the console's rotation panel
// is first offered to an operator who no longer has time to use it.
func TestCAGetsALongerWarningWindowThanAHost(t *testing.T) {
	day := 24 * time.Hour

	if hooks.CAExpiryWindow <= hooks.CertExpiryWindow {
		t.Fatalf("the CA window (%v) must be longer than the host window (%v): "+
			"a CA cannot be renewed, only rotated, and rotation cannot be hurried",
			hooks.CAExpiryWindow, hooks.CertExpiryWindow)
	}

	// 60 days: inside the CA's 90-day window, outside a host's 30-day one.
	between := 60 * day
	f := newCertFixture(t, []time.Duration{between}, []time.Duration{between})

	sums, err := hooks.NebulaCertSummaries(f.app, f.opts, f.now)
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	ca, host := sums[0], sums[1]

	if ca.Expiring != 1 {
		t.Errorf("a CA %v out is not being called out (expiring=%d); by the time it is, "+
			"rotation no longer fits before the deadline", between, ca.Expiring)
	}
	if host.Expiring != 0 {
		t.Errorf("a host certificate %v out was called out (expiring=%d); it renews itself, "+
			"and warning this early makes the badge meaningless", between, host.Expiring)
	}
}

// A decommissioned device's lapsed certificate is not a fault anyone should be
// paged about, and active=false is this platform's decommission flag.
func TestCertExpiryIgnoresDecommissionedHosts(t *testing.T) {
	f := newCertFixture(t, []time.Duration{400 * 24 * time.Hour}, []time.Duration{-5 * 24 * time.Hour})

	col, err := f.app.FindCollectionByNameOrId("nebula_hosts")
	if err != nil {
		t.Fatal(err)
	}
	_ = col
	if _, err := f.app.DB().NewQuery("UPDATE {{nebula_hosts}} SET active = false").Execute(); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	sums, err := hooks.NebulaCertSummaries(f.app, f.opts, f.now)
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if sums[1].Total != 0 || sums[1].Expired != 0 {
		t.Errorf("a deactivated host still counts: total=%d expired=%d, want 0/0", sums[1].Total, sums[1].Expired)
	}
	if res := runCertCheck(t, f); res.State != health.StateOK {
		t.Errorf("state = %q, want ok once the only expired host is decommissioned (detail: %s)", res.State, res.Detail)
	}
}

func TestCertCheckWarnsButNeverFails(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		name      string
		ca, hosts []time.Duration
		want      health.State
		mustSay   string
	}{
		{"all healthy", []time.Duration{400 * day}, []time.Duration{200 * day}, health.StateOK, "valid"},
		{"a host expiring", []time.Duration{400 * day}, []time.Duration{5 * day}, health.StateWarn, "expiring"},
		{"a host expired", []time.Duration{400 * day}, []time.Duration{-1 * day}, health.StateWarn, "expired"},
		// The CA is worth naming separately: every host certificate chains to
		// it, so "1 CA expired" and "1 host certificate expired" are very
		// different mornings.
		{"the CA expired", []time.Duration{-1 * day}, []time.Duration{200 * day}, health.StateWarn, "1 CA expired"},
		{"nothing issued", nil, nil, health.StateSkipped, "no Nebula certificates"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newCertFixture(t, tc.ca, tc.hosts)
			res := runCertCheck(t, f)

			if res.State != tc.want {
				t.Errorf("state = %q, want %q (detail: %s)", res.State, tc.want, res.Detail)
			}
			// The design point: a lapsed device certificate must not take the
			// Control Plane out of a load balancer.
			if res.State == health.StateFail {
				t.Error("a certificate expiry made the process UNREADY -- readiness means 'stop sending me traffic', " +
					"and an expired host certificate is not this process being unable to serve")
			}
			if !strings.Contains(res.Detail, tc.mustSay) {
				t.Errorf("detail %q does not mention %q", res.Detail, tc.mustSay)
			}
			if res.State == health.StateWarn && res.Fix == "" {
				t.Error("a warning with no Fix: these checks exist to be read by someone who does not yet know the remedy")
			}
		})
	}
}

// A row with no readable expiry is not a row known to be valid.
func TestCertCheckDoesNotTreatAMissingExpiryAsHealthy(t *testing.T) {
	f := newCertFixture(t, []time.Duration{400 * 24 * time.Hour}, []time.Duration{0}) // 0 = expires_at left unset

	sums, err := hooks.NebulaCertSummaries(f.app, f.opts, f.now)
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if sums[1].NoExpiry != 1 {
		t.Errorf("host NoExpiry = %d, want 1", sums[1].NoExpiry)
	}

	res := runCertCheck(t, f)
	if res.State != health.StateWarn {
		t.Errorf("state = %q, want warn for a certificate with no readable expiry (detail: %s)", res.State, res.Detail)
	}
}

// Nothing in CI scrapes /metrics, so a malformed body would look completely
// fine in a terminal and be silently unscrapeable. Same reasoning as
// internal/metrics/metrics_test.go, applied to the new collector.
func TestCertMetricsScrapeCleanly(t *testing.T) {
	day := 24 * time.Hour
	f := newCertFixture(t, []time.Duration{400 * day}, []time.Duration{-2 * day, 7 * day})

	set := metrics.New("stone_age", "test")
	hooks.RegisterPlatformMetrics(f.app, set, f.opts)
	body := scrapeMetrics(t, set)

	parser := expfmt.NewTextParser(model.UTF8Validation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(body))
	if err != nil {
		t.Fatalf("exposition does not parse: %v", err)
	}

	for _, want := range []string{
		"stone_age_certificates",
		"stone_age_certificates_expired",
		"stone_age_certificates_expiring",
		"stone_age_certificate_expiry_seconds",
	} {
		if _, ok := families[want]; !ok {
			t.Errorf("missing metric family %q", want)
		}
	}

	problems, err := promlint.New(strings.NewReader(body)).Lint()
	if err != nil {
		t.Fatalf("promlint: %v", err)
	}
	for _, p := range problems {
		t.Errorf("promlint: %s: %s", p.Metric, p.Text)
	}

	// No organization label, anywhere. /metrics is open by default, so a tenant
	// name beside a certificate inventory is free reconnaissance -- the same
	// rule that keeps stone_age_records unlabelled by org.
	for name, fam := range families {
		if !strings.HasPrefix(name, "stone_age_certificate") {
			continue
		}
		for _, m := range fam.Metric {
			for _, l := range m.Label {
				if l.GetName() != "kind" {
					t.Errorf("%s carries label %q=%q; only `kind` is allowed on these series",
						name, l.GetName(), l.GetValue())
				}
			}
		}
	}
}

// The check and the metric are read by different people at different times, and
// the only thing keeping them consistent is that both call the same scan. If
// one ever grows its own query, this is what notices.
func TestCertCheckAndMetricAgree(t *testing.T) {
	day := 24 * time.Hour
	f := newCertFixture(t, []time.Duration{400 * day}, []time.Duration{-2 * day, 7 * day, 500 * day})

	res := runCertCheck(t, f)
	if res.State != health.StateWarn {
		t.Fatalf("expected the check to warn about the expired host, got %q (detail: %s)", res.State, res.Detail)
	}

	set := metrics.New("stone_age", "test")
	hooks.RegisterPlatformMetrics(f.app, set, f.opts)
	body := scrapeMetrics(t, set)

	if !strings.Contains(body, `stone_age_certificates_expired{kind="nebula_host"} 1`) {
		t.Errorf("the check warned about an expired host certificate but the metric does not report one.\n%s",
			grepLines(body, "stone_age_certificates"))
	}
	if !strings.Contains(body, `stone_age_certificates_expiring{kind="nebula_host"} 1`) {
		t.Errorf("expiring count disagrees with the check.\n%s", grepLines(body, "stone_age_certificates"))
	}

	// An absent series is answerable with absent(); a zero timestamp is 1970,
	// which every "expires soon" alert would fire on forever.
	if strings.Contains(body, `stone_age_certificate_expiry_seconds{kind="nebula_ca"} 0`) {
		t.Error("expiry timestamp emitted as 0 -- omit the series instead when there is nothing to report")
	}
}

// scrapeMetrics renders the exposition through the real /metrics handler, so
// what is asserted is the bytes a Prometheus server would actually receive.
func scrapeMetrics(t *testing.T, set *metrics.Set) string {
	t.Helper()
	rec := httptest.NewRecorder()
	set.Handler("").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}
