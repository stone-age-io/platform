package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_floorplan_position re-imports the embedded schema.json so existing
// deployments pick up the new `floorplan_position` json field on `things`.
// Positions used to live in the free-form `metadata` blob, where a user editing
// metadata could silently clobber them; they now have a dedicated field.
// Additive import — fresh DBs that already have the field treat this as a no-op.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping floorplan_position schema update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Added floorplan_position json field to things")
		return nil
	}, nil)
}
