package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// stage2_schema_update re-imports the embedded schema.json so that Stage 1
// deployments (where initial_schema already ran once with the Stage 1 schema)
// pick up the new `thing_type_operations` collection and the `operations` /
// `nats_role` fields on `thing_types`.
//
// ImportCollectionsByMarshaledJSON with deleteMissing=false merges additively,
// so fresh deployments that already imported the Stage 2 schema via
// initial_schema.go treat this as a no-op.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping Stage 2 schema update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Stage 2 schema updates applied")
		return nil
	}, nil)
}
