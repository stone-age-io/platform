package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_floorplan_position re-imports the embedded schema.json so existing
// deployments pick up the new `floorplan_position` json field on `things`.
// Positions lived in the free-form `metadata` blob, where editing metadata could
// silently clobber them.
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
