package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_org_stats adds org_stats, the first view collection in the
// schema: one row per organization carrying four counts (things, active
// things, locations, members) and nothing else.
//
// It lets the operator's organization page show a tenant's size. Operators
// deliberately have no read branch on things, locations or memberships, so
// counting those directly returns 0 (see OrganizationDetailView.vue).
//
// A view's API rule replaces the rules of the tables it reads, so operators get
// exactly its columns. Keep them aggregates only; org_stats_test.go pins the
// column list, because a name or code here would expose tenant inventory to
// every operator. AUTHORIZATION.md has the general form.
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
