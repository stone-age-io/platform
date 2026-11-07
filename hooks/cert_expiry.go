package hooks

import (
	"fmt"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"
)

// Nebula certificate expiry, shared by the readiness check and the metrics
// collector so the two cannot disagree.
//
// The Control Plane may check this because it is answerable first-hand: every
// other credential the platform issues lives where this process cannot look (a
// tenant's NATS account, an edge box's KV), but it signed and stores the Nebula
// certificates itself. That is the bar for a check (package internal/health).
//
// Expiry warns rather than fails: a lapsed certificate is a device that cannot
// join the overlay, not this process unable to serve, and a failed readiness
// would pull the console out of a load balancer.
//
// It is worth having because a CA lapses silently, years after it was minted,
// and takes every host in the mesh down at once.

// certExpiryWindow is the HOST look-ahead. The CA has its own, below.
//
// Mirrors EXPIRY_WARNING_DAYS in ui/src/utils/expiry.ts so the two screens
// agree on what "expiring" means. No cross-language test: a mismatch makes two
// views inconsistent, not silently inert.
const certExpiryWindow = 30 * 24 * time.Hour

// caExpiryWindow is the CA look-ahead, deliberately three times the host's.
//
// Hosts renew themselves, so 30 days is ample. A CA cannot be renewed, only
// rotated, and rotation has a wait in the middle that must outlast every host
// fetching a new config. Nebula's guide says to start two to three months out,
// which is where 90 comes from (pb-nebula warns at 90 too, on a log this
// deployment does not show). 30 days' notice on a CA is notice that the remedy
// no longer fits.
//
// Mirrored by CA_EXPIRY_WARNING_DAYS in ui/src/utils/expiry.ts, and that mirror
// matters more than certExpiryWindow's: only a tenant's owners and admins can
// rotate its CA, and this window only reaches /api/ready and /metrics, the
// operator's surfaces. So the console is the only warning the party who can act
// sees. ui/src/utils/expiry.spec.ts fails if the two console windows collapse.
const caExpiryWindow = 90 * 24 * time.Hour

// certKind values are the metric's `kind` label values, so they are stable.
const (
	certKindCA   = "nebula_ca"
	certKindHost = "nebula_host"
)

// certSummary is one kind's certificate counts and its soonest expiry.
type certSummary struct {
	Kind  string
	Total int
	// Expired is already past; Expiring is within Window and not yet past. They
	// never overlap.
	Expired  int
	Expiring int
	// Earliest is the soonest expiry of this kind, zero when there is none. It
	// is what the metric publishes, with no per-organization label.
	Earliest time.Time
	// NoExpiry counts rows whose expires_at is empty or unparseable. Reported,
	// not treated as healthy.
	NoExpiry int
	// Window is the look-ahead Expiring was counted against, carried so a
	// message cannot quote the host window for a CA.
	Window time.Duration
}

// scanCertExpiry summarises one collection's expires_at column.
//
// The arithmetic is in Go, not SQL date predicates: those only work because
// PocketBase's text date format happens to sort chronologically, and would keep
// returning plausible numbers if it changed. `where` narrows the rows (hosts
// are filtered to active = true).
func scanCertExpiry(app core.App, collection, kind, where string, window time.Duration, now time.Time) (certSummary, error) {
	sum := certSummary{Kind: kind, Window: window}
	if collection == "" {
		return sum, nil
	}

	query := "SELECT expires_at FROM {{" + collection + "}}"
	if where != "" {
		query += " WHERE " + where
	}

	var raw []string
	if err := app.DB().NewQuery(query).Column(&raw); err != nil {
		return sum, fmt.Errorf("read %s.expires_at: %w", collection, err)
	}

	sum.Total = len(raw)
	for _, v := range raw {
		when, err := types.ParseDateTime(v)
		if err != nil || when.IsZero() {
			sum.NoExpiry++
			continue
		}
		at := when.Time()
		if sum.Earliest.IsZero() || at.Before(sum.Earliest) {
			sum.Earliest = at
		}
		switch {
		case !at.After(now):
			sum.Expired++
		case at.Sub(now) <= window:
			sum.Expiring++
		}
	}
	return sum, nil
}

// nebulaCertSummaries scans both Nebula certificate collections, in a fixed
// order (CA first) so the readiness detail and metric series are stable.
func nebulaCertSummaries(app core.App, opts ObservabilityOptions, now time.Time) ([]certSummary, error) {
	specs := []struct {
		collection string
		kind       string
		where      string
		window     time.Duration
	}{
		{opts.NebulaCACollection, certKindCA, "", caExpiryWindow},
		// A decommissioned device's lapsed certificate needs no warning.
		{opts.NebulaHostCollection, certKindHost, "active = true", certExpiryWindow},
	}

	out := make([]certSummary, 0, len(specs))
	for _, s := range specs {
		sum, err := scanCertExpiry(app, s.collection, s.kind, s.where, s.window, now)
		if err != nil {
			return nil, err
		}
		out = append(out, sum)
	}
	return out, nil
}

// certExpiringPhrase is certExpiryPhrase for the Expiring count, naming each
// kind's own window: "2 host certificates within 30 days, 1 CA within 90 days".
func certExpiringPhrase(sums []certSummary) string {
	var parts []string
	for _, s := range sums {
		if s.Expiring == 0 {
			continue
		}
		one := certExpiryPhrase([]certSummary{s}, func(c certSummary) int { return c.Expiring })
		parts = append(parts, fmt.Sprintf("%s within %d days", one, int(s.Window.Hours()/24)))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// certExpiryPhrase names which kinds are affected, e.g. "1 CA, 3 host
// certificates": three devices and the CA every device chains to are very
// different problems.
func certExpiryPhrase(sums []certSummary, pick func(certSummary) int) string {
	var parts []string
	for _, s := range sums {
		n := pick(s)
		if n == 0 {
			continue
		}
		noun := "host certificate"
		if s.Kind == certKindCA {
			noun = "CA"
		}
		if n != 1 {
			noun += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", n, noun))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}
