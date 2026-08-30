package migrations

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"

	"platform/hooks"
)

// orgCodePattern mirrors the `pattern` on organizations.code in schema.json.
// Duplicated rather than derived because the schema is data and this is a
// pre-flight check that has to run before the import validates anything.
var orgCodePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,30}$`)

// schema_update_org_code adds `organizations.code` -- the root of the public
// namespace, and the only globally unique identifier in the ecosystem (ADR 0002
// in platform-docs).
//
// Everything else in the platform already federates on a code: stone-cli
// resolves by it, leaf-sync keys KV by it, twin keys are <kind>.<code>.<prop>,
// and sibling apps resolve a payload thing_code / location_code per
// (organization, code). What was missing was a code for the ORGANIZATION, so
// the one place a tenant had to be named -- the managed-org subject rewrite --
// reached for the PocketBase id instead. That left the ecosystem with two
// namespace roots, and made the tenant token a single database's primary key.
//
// Three details worth keeping:
//
// The index is PARTIAL, matching the five that schema_update_unique_org_code.go
// added. Not a hedge: it is what makes this one migration instead of two. A
// column cannot be backfilled before it exists, and a total unique index cannot
// be imported over existing rows that have no code, because SQLite treats '' as
// a value rather than as NULL. A partial index imports cleanly at any backfill
// state, and hooks.RegisterOrgCode makes blanks unreachable from here forward.
//
// The backfill writes through app.DB() rather than app.Save(). Saving an
// organization fires the managed-export reconciler, which would try to
// provision NATS export/import records in the middle of `migrate up` -- work
// that has nothing to do with adding a column, on a schema that was imported
// seconds ago. The column was just created and its values are validated below,
// so the record layer has nothing to contribute.
//
// The sweep runs BEFORE any write, and it is fatal. This deliberately does not
// resolve collisions by suffixing: the code is immutable once saved, so an
// auto-assigned `acme-2` is permanent, and choosing which org is the real
// `acme` is a decision the platform does not get to make. Same stance, and the
// same reason, as the duplicate refusal in schema_update_unique_org_code.go.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping organizations.code")
			return nil
		}

		// Import first: the column has to exist before it can be read or written.
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		type orgRow struct {
			Id   string `db:"id"`
			Name string `db:"name"`
			Code string `db:"code"`
		}
		var rows []orgRow
		if err := app.DB().
			NewQuery("SELECT [[id]], [[name]], [[code]] FROM {{organizations}}").
			All(&rows); err != nil {
			return fmt.Errorf("read organizations: %w", err)
		}

		// Codes already in the table are frozen and win over anything derived.
		taken := map[string]string{}
		for _, r := range rows {
			if r.Code != "" {
				taken[r.Code] = r.Name
			}
		}

		var problems []string
		derived := map[string]string{}

		for _, r := range rows {
			if r.Code != "" {
				continue
			}
			code := hooks.Slugify(r.Name)
			switch {
			case code == "":
				problems = append(problems, fmt.Sprintf(
					"  %q -> nothing usable could be derived from the name", r.Name))
			case !orgCodePattern.MatchString(code):
				problems = append(problems, fmt.Sprintf(
					"  %q -> derived %q, which is not a valid code (must match %s)",
					r.Name, code, orgCodePattern))
			case taken[code] != "":
				problems = append(problems, fmt.Sprintf(
					"  %q -> derived %q, already used by %q", r.Name, code, taken[code]))
			default:
				taken[code] = r.Name
				derived[r.Id] = code
			}
		}

		if len(problems) > 0 {
			return fmt.Errorf(
				"cannot backfill organizations.code for %d organization(s):\n%s\n\n"+
					"An organization code is the root of the public namespace: it roots the\n"+
					"operator-signed subject rewrite and is printed on physical labels, and it is\n"+
					"immutable once set. Assign codes to these organizations by hand (admin panel\n"+
					"at /_/, or the API), then run `migrate up` again. This migration deliberately\n"+
					"does not pick one for you.",
				len(problems), strings.Join(problems, "\n"))
		}

		for id, code := range derived {
			if _, err := app.DB().
				NewQuery("UPDATE {{organizations}} SET [[code]] = {:code} WHERE [[id]] = {:id}").
				Bind(dbx.Params{"code": code, "id": id}).
				Execute(); err != nil {
				return fmt.Errorf("backfill code %q: %w", code, err)
			}
		}

		log.Printf("✅ organizations.code added and frozen; %d code(s) backfilled", len(derived))
		return nil
	}, func(app core.App) error {
		// Down: no-op. Dropping the column would discard operator-assigned codes
		// that may already be printed on labels and embedded in signed account
		// JWTs, and the field is inert when unused.
		return nil
	})
}
