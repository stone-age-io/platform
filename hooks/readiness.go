package hooks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"platform/internal/health"
)

// dialTimeout bounds each NATS check. A bus that needs longer to answer is
// already the problem being reported.
const dialTimeout = 3 * time.Second

// registerPlatformChecks adds the Control Plane's readiness checks to reg.
//
// Every check is answerable by this process alone: its own database, its own
// configuration, and the NATS server it dials. The Control Plane has no
// credential inside any organization's account, so per-site liveness is
// deliberately absent (the package comment on internal/health).
func registerPlatformChecks(app core.App, reg *health.Registry, opts ObservabilityOptions) {
	reg.Register("database", func(ctx context.Context) health.Result {
		var one int
		err := app.DB().NewQuery("SELECT 1").Row(&one)
		if err != nil {
			return health.Fail(
				fmt.Sprintf("cannot query the database: %v", err),
				"Check that pb_data/ is writable and not held by another running instance.",
			)
		}
		return health.OK("responding")
	})

	// Not redundant with `serve` running migrations first: the initial migration
	// returns nil and marks itself applied when the embedded SchemaJSON is
	// empty. A build whose embed did not take has none of the platform's fields,
	// a migrations table that says everything ran, and silent no-op writes to
	// every platform flag.
	reg.Register("schema", func(ctx context.Context) health.Result {
		missing := MissingFields(app, PlatformSchemaFields(opts.OrgCollection, opts.MembershipCollection))
		if len(missing) > 0 {
			return health.Fail(
				"schema.json has not been imported: missing "+strings.Join(missing, ", "),
				"Run: ./stone-age migrate up",
			)
		}
		return health.OK("platform fields present")
	})

	// A DOWNGRADE: the database has migrations this binary does not know.
	// Deliberately not "migrations pending", which cannot happen because `serve`
	// applies them before it listens; a check that can only report ok means
	// nothing. Migrations do not roll back, so an older binary on a newer
	// pb_data runs against columns and rules it does not know.
	reg.Register("schema_version", func(ctx context.Context) health.Result {
		unknown, err := unknownMigrations(app)
		if err != nil {
			return health.Skip(fmt.Sprintf("could not read the migrations table: %v", err))
		}
		if len(unknown) > 0 {
			return health.Fail(
				fmt.Sprintf("the database has %d migration(s) this binary does not know: %s",
					len(unknown), strings.Join(unknown, ", ")),
				"This pb_data was written by a NEWER build than the one running. Migrations do not roll "+
					"back, so deploy the newer binary again rather than trying to downgrade the data.",
			)
		}
		return health.OK("database and binary agree")
	})

	// A platform with no operator has no one who can create an organization: a
	// dead install that serves 200s.
	reg.Register("bootstrap", func(ctx context.Context) health.Result {
		operator, err := app.FindFirstRecordByFilter("users", "is_operator = true")
		if err != nil || operator == nil {
			return health.Fail(
				"no user has is_operator set",
				"Run: ./stone-age bootstrap --email <you> --org \"System\" --operator-org \"<your company>\"  (after `migrate up`)",
			)
		}
		if _, err := app.FindFirstRecordByFilter(opts.OrgCollection, "is_system_org = true"); err != nil {
			return health.Fail(
				"no organization has is_system_org set",
				"Run: ./stone-age bootstrap --email <you> --org \"System\"",
			)
		}
		return health.OK("operator and system organization present")
	})

	// Without this record nothing can sign an account or user JWT, so every
	// organization created from here on gets no working NATS identity.
	reg.Register("nats_operator", func(ctx context.Context) health.Result {
		op, err := app.FindFirstRecordByFilter(operatorCollection, "1=1")
		if err != nil || op == nil {
			return health.Fail(
				"no NATS operator record",
				"Run: ./stone-age superuser upsert <email> '<password>'  — this seeds the operator and the $SYS account.",
			)
		}
		if op.GetString("jwt") == "" {
			return health.Fail(
				"the NATS operator record has no JWT",
				"Re-run `./stone-age superuser upsert` to re-seed the operator.",
			)
		}
		return health.OK("seeded")
	})

	// Reachability only — see health.DialInfo for why this is separate from the
	// trust check below.
	reg.Register("nats_reachable", func(ctx context.Context) health.Result {
		info, err := health.DialInfo(ctx, opts.NatsServerURL, dialTimeout)
		if err != nil {
			return health.Fail(
				fmt.Sprintf("cannot reach %s: %v", opts.NatsServerURL, err),
				"Start a NATS server there, or run this binary with `serve --nats` to host one in-process. "+
					"Note nats.server_url is the TCP address THIS process dials, not the browser's WebSocket URL.",
			)
		}
		detail := fmt.Sprintf("%s (nats-server %s)", info.ServerName, info.Version)
		if !info.JetStream {
			return health.Warn(
				detail+", JetStream disabled",
				"KV buckets — the digital twin, and everything an edge site mirrors — all need JetStream. "+
					"Enable it in nats.conf.",
			)
		}
		return health.OK(detail + ", JetStream enabled")
	})

	// The failure nothing else shows: a running NATS server that does not trust
	// this database's operator. Account claims are rejected, no organization's
	// account reaches the bus, and the console looks fine.
	reg.Register("nats_trust", func(ctx context.Context) health.Result {
		creds, why := systemUserCreds(app, opts.NatsAccountCollection, opts.NatsUserCollection)
		if creds == "" {
			// Not a failure of the bus: say which credential is missing.
			return health.Skip("cannot test: " + why)
		}
		if err := health.CheckCreds(opts.NatsServerURL, creds, dialTimeout); err != nil {
			return health.Fail(
				fmt.Sprintf("the NATS server at %s rejected this platform's $SYS credential: %v", opts.NatsServerURL, err),
				"The server's operator JWT does not match the operator in this database. "+
					"Regenerate its config with `./stone-age nats export --output ./nats-config/` and restart nats-server.",
			)
		}
		return health.OK("the NATS server trusts this platform's operator")
	})

	// Nebula certificates this process signed and stores. See hooks/cert_expiry.go
	// for why the Control Plane may check these when it may not check a tenant's
	// KV, and why an expiry warns rather than fails.
	reg.Register("nebula_cert_expiry", func(ctx context.Context) health.Result {
		sums, err := nebulaCertSummaries(app, opts, time.Now())
		if err != nil {
			return health.Skip(fmt.Sprintf("could not read certificate expiry: %v", err))
		}

		var total, expired, expiring, unreadable int
		var earliest time.Time
		for _, s := range sums {
			total += s.Total
			expired += s.Expired
			expiring += s.Expiring
			unreadable += s.NoExpiry
			if !s.Earliest.IsZero() && (earliest.IsZero() || s.Earliest.Before(earliest)) {
				earliest = s.Earliest
			}
		}

		// No certificates means Nebula is not in use: a check that did not
		// apply, which is why skipped ranks below ok.
		if total == 0 {
			return health.Skip("no Nebula certificates issued")
		}

		if expired > 0 {
			return health.Warn(
				fmt.Sprintf("%s expired (%d of %d certificates)", certExpiryPhrase(sums, func(s certSummary) int { return s.Expired }), expired, total),
				"Re-issue them: a host with a lapsed certificate cannot join the overlay, and an expired CA "+
					"invalidates every certificate under it. Raise validity_years when re-issuing so the next "+
					"lapse is not on the same schedule.",
			)
		}
		if expiring > 0 {
			return health.Warn(
				fmt.Sprintf("expiring: %s (soonest %s)",
					certExpiringPhrase(sums), earliest.UTC().Format(time.DateOnly)),
				"Re-issue before the date above. Nebula certificates fail all at once and silently — "+
					"nothing retries, and the devices simply stop appearing.",
			)
		}
		if unreadable > 0 {
			return health.Warn(
				fmt.Sprintf("%d certificate(s) have no readable expiry date", unreadable),
				"A certificate whose expires_at is empty is not one known to be valid. Re-issue it, "+
					"or check whether it was created outside the normal provisioning path.",
			)
		}
		return health.OK(fmt.Sprintf("%d valid, soonest expiry %s", total, earliest.UTC().Format(time.DateOnly)))
	})

	// Configuration warnings: neither blocks serving.
	reg.Register("nats_websocket_urls", func(ctx context.Context) health.Result {
		if len(opts.NatsWebsocketURLs) == 0 {
			return health.Warn(
				"nats.websocket_urls is empty, so the console falls back to ws://localhost:9222",
				"Set nats.websocket_urls (or STONE_AGE_NATS_WEBSOCKET_URLS) to the WebSocket address a BROWSER can reach. "+
					"Anyone loading the console from another machine cannot connect to the bus until you do.",
			)
		}
		return health.OK(fmt.Sprintf("%d configured", len(opts.NatsWebsocketURLs)))
	})

	reg.Register("encryption_at_rest", func(ctx context.Context) health.Result {
		var off []string
		if opts.NatsEncryptionKey == "" {
			off = append(off, "nats")
		}
		if opts.NebulaEncryptionKey == "" {
			off = append(off, "nebula")
		}
		switch len(off) {
		case 0:
			// Says what it covers: creds_file and config_yaml are plaintext by
			// construction (SECURITY.md).
			return health.OK("enabled for NATS and Nebula (minting keys; issued credentials are plaintext by construction)")
		case 2:
			return health.Warn(
				"disabled: NATS seeds and Nebula private keys are stored in plaintext",
				"Set nats.encryption_key and nebula.encryption_key to exactly 32 characters "+
					"(via STONE_AGE_*_ENCRYPTION_KEY). Back the keys up — losing one loses the encrypted records.",
			)
		default:
			return health.Warn(
				"disabled for "+off[0]+" (enabled for the other)",
				"Set the missing key to exactly 32 characters, or clear both if plaintext at rest is intended.",
			)
		}
	})
}

// unknownMigrations returns applied migration files that this binary does not
// have registered — the signature of a database written by a newer build.
//
// It compares against core.AppMigrations AND core.SystemMigrations, the two
// lists `serve` runs, so it cannot drift from what the binary applies.
// PocketBase's system migrations share the table.
func unknownMigrations(app core.App) ([]string, error) {
	var applied []string
	if err := app.DB().NewQuery("SELECT file FROM {{_migrations}}").Column(&applied); err != nil {
		return nil, err
	}

	known := make(map[string]bool)
	for _, list := range []*core.MigrationsList{&core.AppMigrations, &core.SystemMigrations} {
		for _, m := range list.Items() {
			known[m.File] = true
		}
	}

	var unknown []string
	for _, f := range applied {
		if !known[f] {
			unknown = append(unknown, f)
		}
	}
	return unknown, nil
}

// systemUserCreds returns the $SYS user's .creds contents, or an empty string
// and the reason it could not be found.
//
// It finds the account by name and the user by nats_username, the same keys
// pb-nats uses (getOperatorAndSystemAccount, getSystemUser). It reads
// creds_file rather than the seed because at-rest encryption does not cover it,
// so this needs no second copy of pb-nats's AES handling.
func systemUserCreds(app core.App, accountCollection, userCollection string) (creds string, why string) {
	account, err := app.FindFirstRecordByFilter(
		accountCollection,
		"name = {:name}",
		dbx.Params{"name": systemAccountName},
	)
	if err != nil || account == nil {
		return "", "no $SYS account record (has `superuser upsert` been run?)"
	}

	users, err := app.FindAllRecords(userCollection, dbx.HashExp{
		"nats_username": "sys",
		"account_id":    account.Id,
	})
	if err != nil || len(users) == 0 {
		return "", "no $SYS user record (has `superuser upsert` been run?)"
	}

	creds = users[0].GetString("creds_file")
	if creds == "" {
		return "", "the $SYS user has no creds_file"
	}
	return creds, ""
}
