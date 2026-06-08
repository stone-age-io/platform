package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_leaf_sync_collections re-imports schema.json so existing
// deployments pick up the leaf_nodes read grants added to thing_type_operations
// and message_schemas. leaf-sync mirrors these so an edge node can resolve the
// full thing_type -> operation -> message_schema graph locally. Both rules keep
// the existing users clause and add the same org-scoped leaf_nodes clause the
// other synced collections already use. Additive import — safe on fresh DBs.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping leaf-sync collections update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Granted leaf_nodes read access to thing_type_operations and message_schemas")
		return nil
	}, nil)
}
