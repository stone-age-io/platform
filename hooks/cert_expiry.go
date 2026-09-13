package hooks

import (
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Nebula certificate expiry, shared by the readiness check and the metrics
// collector so the two cannot disagree.
//
// WHY THE CONTROL PLANE MAY CHECK THIS AT ALL. Every other credential this
// platform issues lives somewhere it cannot look: an organization's NATS
// account, an edge box's KV. Nebula certificates are different — this process
// signed them and stores them, expiry included, in its own database. So it is
// answerable first-hand, which is the whole bar for a check here (see the
// package comment on internal/health).
//
// WHY EXPIRY WARNS RATHER THAN FAILS. An expired host certificate is a device
// that cannot join the overlay. It is not this process being unable to serve,
// and readiness failing means "stop sending me traffic" — so failing here would
// pull the console out of a load balancer because a device certificate lapsed.
// Same reasoning as an islanded edge warning rather than failing.
//
// WHY IT IS WORTH HAVING. A Nebula certificate expires silently and on a
// schedule nobody is watching: the CA is minted at organization-create with a
// multi-year validity, so the day it lapses is years after the only person who
// knew about it stopped thinking about it, and the failure is every host in the
// mesh at once. Until now the only surface for this was ExpiryBadge on a list
// view — visible to someone already looking at the right screen.

// certExpiryWindow is how far ahead a certificate starts being called out.
//
// Mirrors EXPIRY_WARNING_DAYS in ui/src/utils/expiry.ts, deliberately: the
// console badges a host at 30 days and an operator comparing the two screens
// should not find them disagreeing about what "expiring" means. Not worth a
// cross-language test the way the managed-export name is — a mismatch here
// makes two views inconsistent, not silently inert.
//
// This is the HOST window. The CA gets its own, below.
const certExpiryWindow = 30 * 24 * time.Hour

// caExpiryWindow is the same idea for the CA, and it is deliberately much
// longer.
//
// A host certificate is renewable — pb-nebula re-issues one automatically once
// it has burned through its lifetime, so 30 days' notice is ample. A CA cannot
// be renewed at all. The only remedy is rotation, and rotation is a three-step
// procedure with a wait in the middle that nothing can compress: it has to
// outlast every host fetching a config nobody told it to fetch. Nebula's own
// guide asks you to begin two to three months out, which is where 90 comes from
// — and it is what pb-nebula warns at too, on a log line this deployment never
// sees, because nebula.log_to_console is false here.
//
// So these are not a tunable and a copy of it. 30 days' notice on a CA is
// notice that the remedy no longer fits.
const caExpiryWindow = 90 * 24 * time.Hour

// certKind labels one family of certificate. These are the metric's `kind`
// label values, so they are snake_case and stable.
const (
	certKindCA   = "nebula_ca"
	certKindHost = "nebula_host"
)

// certSummary is what both consumers need: how many there are, how many are
// already dead, how many are about to be, and when the next one goes.
type certSummary struct {
	Kind  string
	Total int
	// Expired is already past; Expiring is within certExpiryWindow and not yet
	// past. The two never overlap, so Total-Expired-Expiring is the healthy
	// remainder.
	Expired  int
	Expiring int
	// Earliest is the soonest expiry of any certificate of this kind, zero when
	// there are none (or none carries a parseable date). This is the value the
	// metric publishes: one number per kind that an alert can be written
	// against, with no per-organization label — a tenant name next to a
	// certificate inventory is the same disclosure the metrics package refuses
	// everywhere else.
	Earliest time.Time
	// NoExpiry counts rows whose expires_at is empty or unparseable. Reported
	// rather than treated as healthy: a certificate whose expiry we cannot read
	// is not a certificate we know to be valid.
	NoExpiry int
}

// scanCertExpiry summarises one collection's expires_at column.
//
// It reads the column and does the arithmetic in Go rather than asking SQLite
// for MIN/COUNT with date predicates. PocketBase stores dates as text
// ("2026-01-02 15:04:05.000Z"), which happens to sort lexicographically the
// same way it sorts chronologically, so the SQL version works — but it works by
// coincidence of the storage format, and it would keep returning plausible
// numbers if that format ever changed. One string per row is cheap at any
// inventory size this platform holds.
//
// `where` narrows the rows: host certificates are filtered to active = true,
// because a decommissioned device's lapsed certificate is not a problem anyone
// needs paging about.
func scanCertExpiry(app core.App, collection, kind, where string, window time.Duration, now time.Time) (certSummary, error) {
	sum := certSummary{Kind: kind}
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

// nebulaCertSummaries scans both Nebula certificate collections.
//
// Returned in a fixed order (CA first) so the readiness detail string and the
// metric series are stable between scrapes rather than map-ordered.
func nebulaCertSummaries(app core.App, opts ObservabilityOptions, now time.Time) ([]certSummary, error) {
	specs := []struct {
		collection string
		kind       string
		where      string
		window     time.Duration
	}{
		{opts.NebulaCACollection, certKindCA, "", caExpiryWindow},
		// Only certificates that are meant to be in service. `active = false`
		// is this platform's decommission flag, and hooks/active_flag.go has
		// already killed the NATS half of such a device's identity.
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

// certExpiryPhrase names WHICH kinds are affected, e.g. "1 CA, 3 host
// certificates". The count alone reads the same whether it is three devices or
// the certificate authority every device chains to, and those are very
// different mornings.
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
