package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_operator_org re-imports the embedded schema.json so existing
// deployments pick up the new organization flags (`is_system_org`,
// `is_operator_org`, `managed`), their at-most-one partial unique indexes,
// and the updateRule guard that keeps org owners from self-setting them.
// Additive import — fresh DBs that already have the fields treat this as a no-op.
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
