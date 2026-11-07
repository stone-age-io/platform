package migrations

import (
	"fmt"
	"log"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"

	"platform/hooks"
)

// schema_update_org_code adds `organizations.code`, the root of the public
// namespace and the only globally unique identifier in the ecosystem (ADR 0002
// in platform-docs). Without it the managed-org subject rewrite named a tenant
// by its PocketBase id, a single database's primary key.
//
// The index is partial, like the five in schema_update_unique_org_code.go, so it
// imports over existing rows with no code (SQLite treats an empty string as
// a value, not NULL). hooks.RegisterOrgCode keeps blanks out from here on.
//
// The backfill writes through app.DB(), not app.Save(): saving an organization
// fires the managed-export reconciler, which would provision NATS records in the
// middle of `migrate up`.
//
// Every row is checked before any write, and a problem is fatal. Collisions are
// not suffixed: the code is immutable, so an auto-assigned `acme-2` would be
// permanent, and choosing the real `acme` is not the platform's decision.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping organizations.code")
			return nil
		}

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
			case !hooks.OrgCodePattern.MatchString(code):
				problems = append(problems, fmt.Sprintf(
					"  %q -> derived %q, which is not a valid code (must match %s)",
					r.Name, code, hooks.OrgCodePattern))
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
					"immutable once set, so this migration deliberately does not pick one for you.\n\n"+
					"Note that there is no `code` field to fill in yet. PocketBase runs the whole\n"+
					"migration list inside one transaction (core/migrations_runner.go), so this\n"+
					"failure rolled back the column along with everything else. Rename the\n"+
					"organization to something a valid code can be derived from, run `migrate up`\n"+
					"again, and rename it back afterwards if you like -- `name` stays mutable, and\n"+
					"`code` is frozen only once it has a value.",
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
		// No-op: dropping the column would discard codes already printed on
		// labels and embedded in signed account JWTs.
		return nil
	})
}
