package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_org_stats adds org_stats, the first view collection in the
// schema: one row per organization carrying four counts (things, active
// things, locations, members) and nothing else.
//
// It exists so the operator's organization page can state how big a tenant is.
// An operator has no read branch on things, locations or memberships, and that
// is deliberate: an operator reads organization metadata and infrastructure,
// not tenant inventory. The page once counted anyway and got 0 for every
// organization but the reader's own, because a list rule is applied as an
// extra WHERE and fails silently (the comment in OrganizationDetailView.vue).
//
// A view's API rule REPLACES the rules of the tables it reads, so this grants
// operators exactly what its columns expose. That is why the columns are
// aggregates only and why org_stats_test.go pins the column list: a name or a
// code added here would be tenant inventory readable by every operator, with
// no row rule anywhere to stop it. AUTHORIZATION.md has the general form.
//
// Adding a collection, so a re-import is sufficient.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping org_stats view")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ org_stats view: per-organization counts, operator-read only")
		return nil
	}, nil)
}
