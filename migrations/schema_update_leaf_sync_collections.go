package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_leaf_sync_collections adds the org-scoped leaf_nodes read branch
// to thing_type_operations and message_schemas, so an edge node could resolve
// the thing_type -> operation -> message_schema graph locally.
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
