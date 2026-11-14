package migrations

import (
	"fmt"
	"log"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// codeScopedCollections are the collections whose `code` is a per-organization
// handle rather than a free-text label. leaf-sync and stone-cli resolve records
// by code, and on a duplicate leaf-sync silently falls back to the record id, so
// lookups such as `thing.S01` stop finding anything.
var codeScopedCollections = []string{
	"things",
	"locations",
	"thing_types",
	"location_types",
}

// schema_update_unique_org_code adds UNIQUE (organization, code) to the
// collections whose code is a lookup handle.
//
// `code` was used as a unique key (KV key, subject segment, `stone thing get`)
// but nothing enforced it. leaf_nodes was in the list until
// schema_update_drop_leaf_nodes.go removed that collection.
//
// The index is partial, over non-empty codes only. `code` is optional, and
// SQLite treats an empty string as a value, so a plain index would allow only
// one blank-coded record per organization.
//
// The duplicate check runs before the import and is fatal: SQLite will not build
// a unique index over violating data, and its error names no rows. Duplicates
// are not auto-resolved, since only a person knows which record is the real S01.
//
// Scoped per organization, not globally: two tenants may both have "HQ".
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			return errNoSchema
		}

		var problems []string

		for _, table := range codeScopedCollections {
			type dup struct {
				Organization string `db:"organization"`
				Code         string `db:"code"`
				N            int    `db:"n"`
			}
			var dups []dup

			// A missing table is a fresh database: the import below creates it,
			// and an empty table cannot hold a duplicate.
			err := app.DB().NewQuery(
				"SELECT [[organization]], [[code]], COUNT(*) AS n FROM {{" + table + "}} " +
					"WHERE [[code]] IS NOT NULL AND [[code]] != '' " +
					"GROUP BY [[organization]], [[code]] HAVING n > 1",
			).All(&dups)
			if err != nil {
				log.Printf("ℹ️ Skipped %s duplicate check (table not present yet): %v", table, err)
				continue
			}

			for _, d := range dups {
				problems = append(problems, fmt.Sprintf(
					"  %s: code %q appears %d times in organization %s",
					table, d.Code, d.N, d.Organization))
			}
		}

		if len(problems) > 0 {
			return fmt.Errorf(
				"cannot add UNIQUE (organization, code): existing duplicates must be resolved first.\n%s\n\n"+
					"Each code has to be unique within its organization, because the agent and stone-cli\n"+
					"both resolve records by it -- a duplicate silently changes the KV key shape at the edge\n"+
					"rather than failing. Rename the records that should not own the code (admin panel at /_/,\n"+
					"or the API), then run `migrate up` again. This migration deliberately does not pick a\n"+
					"winner for you.",
				strings.Join(problems, "\n"))
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ UNIQUE (organization, code) applied to things, locations, thing_types, location_types")
		return nil
	}, nil)
}
