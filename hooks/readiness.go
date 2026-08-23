package hooks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"platform/internal/health"
)

// dialTimeout bounds each NATS check. Short: these run on a background prober
// with its own deadline, and a bus that needs more than this to answer is
// already the problem being reported.
const dialTimeout = 3 * time.Second

// registerPlatformChecks adds the Control Plane's readiness checks to reg.
//
// Every check here is answerable by this process alone — its own database, its
// own configuration, and the NATS server it is itself configured to dial. See
// the package comment on internal/health for why that boundary is not an
// accident: the Control Plane holds the operator and $SYS, and has no
// credential inside any organization's account, so per-site liveness is
// deliberately absent from this list.
func registerPlatformChecks(app *pocketbase.PocketBase, reg *health.Registry, opts ObservabilityOptions) {
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

	// This check looks redundant and is not. `serve` calls RunAllMigrations
	// before it listens (PocketBase apis/serve.go), so the schema is normally
	// imported by the time anything can probe — but the initial migration
	// returns nil when the embedded SchemaJSON is empty, logging a warning and
	// marking itself applied. A build whose //go:embed did not take therefore
	// produces a database with the libraries' collections, none of the
	// platform's fields, and a migrations table that says everything ran. In
	// that state record.Set() on a platform flag is a silent no-op, which is
	// how `bootstrap` used to "succeed" while writing nothing.
	reg.Register("schema", func(ctx context.Context) health.Result {
		missing := missingSchemaFields(app, []collectionFields{
			{"users", []string{"is_operator"}},
			{opts.OrgCollection, []string{"is_system_org", "is_operator_org"}},
			{opts.MembershipCollection, []string{"role"}},
		})
		if len(missing) > 0 {
			return health.Fail(
				"schema.json has not been imported: missing "+strings.Join(missing, ", "),
				"Run: ./stone-age migrate up",
			)
		}
		return health.OK("platform fields present")
	})

	// A DOWNGRADE: the database has migrations this binary has never heard of,
	// so its schema is ahead of the code driving it.
	//
	// Deliberately not the other direction. "Migrations pending" cannot happen
	// here — `serve` calls RunAllMigrations before it listens, so anything
	// outstanding is applied before this check could ever run, and a check that
	// can only ever report ok is a green tick that means nothing. Ahead is the
	// half that `serve` cannot fix: migrations do not roll back on startup, so
	// an older binary pointed at a newer pb_data keeps running against columns
	// and rules it does not know about, and the symptom is arbitrary.
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

	// `bootstrap` is the only way to mint an operator outside the admin panel,
	// and running it before `migrate up` used to "succeed" while writing none of
	// the flags. A platform with no operator has no one who can create an
	// organization, which is a dead install that serves 200s.
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
				"KV buckets — digital twin, leaf-node config mirrors, leaf_status — all need JetStream. "+
					"Enable it in nats.conf.",
			)
		}
		return health.OK(detail + ", JetStream enabled")
	})

	// The silent killer: a running NATS server that does not trust this
	// database's operator. Account claims are published and rejected, no
	// organization's account ever reaches the bus, and the console looks fine.
	reg.Register("nats_trust", func(ctx context.Context) health.Result {
		creds, why := systemUserCreds(app, opts.NatsAccountCollection, opts.NatsUserCollection)
		if creds == "" {
			// Not a failure of the bus: we could not find the credential to
			// test with. Say which, rather than blaming NATS.
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

	// Configuration warnings. Neither blocks serving, and both describe a
	// deployment that works today and will embarrass someone later.
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
			return health.OK("enabled for NATS and Nebula")
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

// collectionFields pairs a collection with the fields a check expects on it.
type collectionFields struct {
	collection string
	fields     []string
}

// missingSchemaFields returns "collection.field" for each expected field that is
// not present. A missing collection reports all of its fields, which reads
// better than a separate "collection not found" case.
func missingSchemaFields(app core.App, want []collectionFields) []string {
	var missing []string
	for _, cf := range want {
		col, err := app.FindCollectionByNameOrId(cf.collection)
		if err != nil || col == nil {
			for _, f := range cf.fields {
				missing = append(missing, cf.collection+"."+f)
			}
			continue
		}
		for _, f := range cf.fields {
			if col.Fields.GetByName(f) == nil {
				missing = append(missing, cf.collection+"."+f)
			}
		}
	}
	return missing
}

// unknownMigrations returns applied migration files that this binary does not
// have registered — the signature of a database written by a newer build.
//
// It compares against core.AppMigrations AND core.SystemMigrations, the two
// lists `serve` actually runs, so the comparison can never drift from what the
// binary would apply: a migration file added to the repo joins that list the
// moment it compiles. PocketBase's own system migrations share the table, so
// leaving them out would flag every one of them as unknown.
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
// It locates the account by name and the user by nats_username, matching how
// pb-nats itself resolves them (its getOperatorAndSystemAccount and
// getSystemUser). Using the same keys is what keeps the two in step.
//
// creds_file is deliberately the field read rather than the seed: it is not one
// of the columns at-rest encryption covers, so this needs no decryption code —
// and duplicating pb-nats's AES handling here would be a second implementation
// of the one thing that must never disagree.
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
