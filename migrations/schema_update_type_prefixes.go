package migrations

import (
	"fmt"
	"log"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_type_prefixes applies ADR 0003 (platform-docs) to the schema:
//
//   - UNIQUE (organization, code COLLATE NOCASE) on things, locations,
//     thing_types and location_types, replacing the case-sensitive indexes from
//     schema_update_unique_org_code.go. Codes keep the case they were entered
//     in, and subjects use that stored spelling exactly -- NATS is
//     case-sensitive and nothing here changes that. What changes is which
//     records may EXIST: `cam-1` next to `CAM-1` is two identities, two subject
//     namespaces and two permission sets that nobody can tell apart when the
//     code is read aloud, and because each works on its own nothing ever errors.
//     SQLite's NOCASE folds ASCII only, which is all CodePattern admits.
//   - `prefix` on thing_types and location_types: optional, ^[A-Z]{1,4}$, with a
//     partial unique index per collection. hooks/codes.go copies it into
//     generated codes and keeps the two collections' prefixes apart.
//   - `type` frozen once set on things and locations, in the update rules.
//
// The case sweep runs BEFORE the import and is fatal, for the reason
// schema_update_unique_org_code.go gives: SQLite refuses to build a unique index
// over data that violates it, and a bare "UNIQUE constraint failed" names no
// rows. Nothing here picks a winner. Which of `hq` and `HQ` is the real site is
// a decision about printed labels and signed subjects, and codes are frozen, so
// the fix is a superuser's.
//
// No backfill of blank codes. A code is frozen the moment it exists, so a
// migration that generated codes for existing blank records would be choosing
// permanent identifiers on nobody's behalf. Blank records are counted instead;
// from this migration on, the create hook means no new one can appear.
func init() {
	m.Register(ApplyTypePrefixes, nil)
}

// ApplyTypePrefixes is the migration body. Exported so a test can restore the
// case-sensitive indexes, insert a case-only duplicate, and prove the sweep
// refuses it: a fresh database imports the already-folded schema.json, so
// `migrate up` alone reaches this with nothing to find.
func ApplyTypePrefixes(app core.App) error {
	if len(SchemaJSON) == 0 {
		log.Println("⚠️ SchemaJSON is empty, skipping type prefixes")
		return nil
	}

	var problems []string
	for _, table := range codeScopedCollections {
		type dup struct {
			Organization string `db:"organization"`
			Codes        string `db:"codes"`
		}
		var dups []dup

		// A missing table is a fresh database, which cannot hold a duplicate.
		err := app.DB().NewQuery(
			"SELECT [[organization]], GROUP_CONCAT([[code]], ', ') AS codes FROM {{" + table + "}} " +
				"WHERE [[code]] IS NOT NULL AND [[code]] != '' " +
				"GROUP BY [[organization]], LOWER([[code]]) HAVING COUNT(*) > 1",
		).All(&dups)
		if err != nil {
			log.Printf("ℹ️ Skipped %s case check (table not present yet): %v", table, err)
			continue
		}
		for _, d := range dups {
			problems = append(problems, fmt.Sprintf(
				"  %s: codes %s differ only by case in organization %s",
				table, d.Codes, d.Organization))
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf(
			"cannot make code uniqueness case-insensitive: these codes differ only by case.\n%s\n\n"+
				"Each pair is two records a person cannot tell apart by reading the code aloud,\n"+
				"with two different NATS subjects behind them. Codes are frozen, so correct the one\n"+
				"that should not own the spelling in the admin panel at /_/, then run `migrate up`\n"+
				"again. This migration deliberately does not pick a winner for you.",
			strings.Join(problems, "\n"))
	}

	if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
		return err
	}

	for _, table := range []string{"things", "locations"} {
		n, err := app.CountRecords(table)
		if err != nil {
			continue
		}
		blank, err := app.CountRecords(table, dbx.NewExp("[[code]] = '' OR [[code]] IS NULL"))
		if err == nil && blank > 0 {
			log.Printf("ℹ️ %d of %d %s have no code. They keep working; new ones get a generated code (ADR 0003).", blank, n, table)
		}
	}

	log.Println("✅ code uniqueness ignores case; prefix added to thing_types and location_types; type frozen once set on things and locations")
	return nil
}
