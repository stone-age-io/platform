package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_stage2 re-imports schema.json so Stage 1 deployments pick up the
// `thing_type_operations` collection and the `operations` / `nats_role` fields
// on `thing_types`.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			return errNoSchema
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Stage 2 schema updates applied")
		return nil
	}, nil)
}
