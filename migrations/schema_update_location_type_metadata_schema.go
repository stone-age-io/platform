package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_location_type_metadata_schema adds
// `location_types.metadata_schema`, the locations-side counterpart to the field
// schema_update_thing_type_metadata_schema added on thing_types.
//
// Same semantics; see that migration.
//
// A separate migration, not an edit to that one: an applied migration is never
// re-run, so a field added to it would never reach those databases.
// Nullable and unreferenced by any rule, so no backfill.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			return errNoSchema
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ location_types.metadata_schema added")
		return nil
	}, func(app core.App) error {
		// Down: no-op. Dropping the column would discard authored schemas.
		return nil
	})
}
