package migrations

import (
	"fmt"
	"log"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_type_prefixes applies ADR 0003 (platform-docs) to the schema:
//
//   - UNIQUE (organization, code COLLATE NOCASE) on things, locations,
//     thing_types and location_types, replacing the case-sensitive indexes from
//     schema_update_unique_org_code.go. Codes keep their case and subjects use
//     it exactly (NATS is case-sensitive); what changes is that `cam-1` and
//     `CAM-1` cannot both exist. NOCASE folds ASCII only, which is all
//     CodePattern admits.
//   - `prefix` on thing_types and location_types: optional, ^[A-Z]{1,4}$, with a
//     partial unique index per collection. hooks/codes.go copies it into
//     generated codes.
//   - `type` frozen once set on things and locations, in the update rules.
//
// The case check runs before the import and is fatal: SQLite will not build a
// unique index over violating data, and its error names no rows. Codes are
// frozen, so choosing between `hq` and `HQ` is a superuser's job.
//
// No backfill of blank codes, since a generated code would be permanent. They
// are counted instead; the create hook prevents new ones.
func init() {
	m.Register(ApplyTypePrefixes, nil)
}

// ApplyTypePrefixes is the migration body. It is exported so a test can restore
// the case-sensitive indexes and insert a case-only duplicate, since a fresh
// database has nothing for the check to find.
func ApplyTypePrefixes(app core.App) error {
	if len(SchemaJSON) == 0 {
		return errNoSchema
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
