package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_thing_type_metadata_schema adds `thing_types.metadata_schema`,
// an optional JSON Schema describing the inventory fields tracked for a class
// of device (service dates, asset tags, warranty references).
//
// The Thing form renders typed inputs from it (ui/src/components/common/
// MetadataEditor.vue). No backfill: the field is nullable and no API rule reads
// it, and a type with no schema falls back to free-form key/value rows.
//
// The schema drives rendering only. Nothing validates `things.metadata` against
// it, on purpose: editing a type's schema would otherwise invalidate existing
// Things and block saves the member cannot fix.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping thing_types.metadata_schema")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ thing_types.metadata_schema added")
		return nil
	}, func(app core.App) error {
		// No-op: dropping the column would discard every authored schema.
		return nil
	})
}
