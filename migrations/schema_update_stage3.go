package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_stage3 re-imports the embedded schema.json so that earlier
// deployments pick up the new `message_schemas` collection and the `schema`
// relation field on `thing_type_operations`. Additive; safe on fresh DBs.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping Stage 3 schema update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Stage 3 schema updates applied")
		return nil
	}, nil)
}
