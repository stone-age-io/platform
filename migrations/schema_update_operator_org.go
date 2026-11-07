package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_operator_org re-imports schema.json for the organization flags
// `is_system_org`, `is_operator_org` and `managed`, their at-most-one partial
// unique indexes, and the updateRule guard that keeps org owners from setting
// them.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping operator org schema update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Added operator/system/managed flags to organizations")
		return nil
	}, nil)
}
