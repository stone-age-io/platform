package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_org_logo re-imports the embedded schema.json so existing
// deployments pick up the new `logo` file field on `organizations`.
// Additive import — fresh DBs that already have the field treat this as a no-op.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping org logo schema update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Added logo file field to organizations")
		return nil
	}, nil)
}
