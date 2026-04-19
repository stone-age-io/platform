package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_org_scope re-imports schema.json so deployments that ran Stage 3
// pick up the tightened list/view rules on thing_type_operations and
// message_schemas (dropped the now-unused `organization = ""` platform-shipped
// clause). Additive import — safe on fresh DBs.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping org-scope rule update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Tightened thing_type_operations + message_schemas rules to current org only")
		return nil
	}, nil)
}
