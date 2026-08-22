package migrations

import (
	"fmt"
	"log"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// codeScopedCollections are the collections whose `code` is a per-organization
// handle rather than a free-text label.
//
// The common thread is that something outside PocketBase resolves a record BY
// its code. `leaf-sync` keys every mirrored record in NATS KV by the handle
// `candidateKey` derives -- composite, then code, then name -- and `stone-cli`
// resolves entities the same way, so a duplicate code means two records
// competing for one KV key. The mirror does not fail: `recordKey` falls back to
// the record id when a code is shared, so the key silently changes shape and
// every consumer looking up `thing.S01` stops finding anything. A digital twin
// key prefix has the same problem, one layer up.
var codeScopedCollections = []string{
	"things",
	"locations",
	"thing_types",
	"location_types",
	"leaf_nodes",
}

// schema_update_unique_org_code adds UNIQUE (organization, code) to the five
// collections whose code is a lookup handle, and freezes `code` on leaf_nodes.
//
// Nothing enforced this before. `code` was documented as unique and used as one
// -- as a KV key, as a subject segment, as the argument to `stone thing get` --
// while the database happily accepted two Things with the same code in the same
// organization, and the behaviour that followed was silent rather than loud.
//
// Three details worth keeping:
//
// The index is PARTIAL: it covers only rows whose code is not the empty
// string. `code` is optional on all five collections, and SQLite treats the
// empty string as a value rather than as NULL, so a plain
// composite unique index would let an organization have exactly one record with
// a blank code. That would be a new restriction nobody asked for, and it would
// surface as "failed to create location" on the second blank-coded row.
// PocketBase's own email indexes use the same partial form.
//
// The sweep runs BEFORE the import, and it is fatal. SQLite refuses to build a
// unique index over data that violates it, so without the check the migration
// fails with a bare "UNIQUE constraint failed" naming no rows. Refusing with the
// actual duplicates listed is the difference between a five-minute fix and an
// afternoon. This is deliberately not auto-resolved: renaming somebody's records
// is a decision about which one is the real S01, and the platform does not know.
//
// Scoped per organization, not globally. Two tenants both calling a site "HQ" is
// correct and expected -- the whole point of the tenancy model is that they do
// not collide. This mirrors nebula_hosts, which already carries
// UNIQUE (network_id, hostname).
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping unique org/code update")
			return nil
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
					"Each code has to be unique within its organization, because leaf-sync and stone-cli\n"+
					"both resolve records by it -- a duplicate silently changes the KV key shape at the edge\n"+
					"rather than failing. Rename the records that should not own the code (admin panel at /_/,\n"+
					"or the API), then run `migrate up` again. This migration deliberately does not pick a\n"+
					"winner for you.",
				strings.Join(problems, "\n"))
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ UNIQUE (organization, code) applied to things, locations, thing_types, location_types, leaf_nodes; leaf_nodes.code frozen")
		return nil
	}, nil)
}
