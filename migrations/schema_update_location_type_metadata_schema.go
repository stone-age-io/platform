package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_location_type_metadata_schema adds
// `location_types.metadata_schema`, the locations-side counterpart to the field
// schema_update_thing_type_metadata_schema added on thing_types.
//
// Same field, same semantics, same reasons — see that migration for the
// argument. Locations carry inventory metadata too (a floor's last inspection,
// a site's access notes), and a location type is the natural place to say what
// is tracked for a class of place.
//
// This is a SEPARATE migration rather than an edit to the thing_types one on
// purpose. Every migration here re-imports the whole of schema.json, so
// widening the earlier file would have worked only on databases that had not
// run it yet — anywhere it was already applied, PocketBase records it as done
// and skips it, and the new field would silently never appear. A new file is
// correct regardless of what a given database has already seen.
//
// Nullable, no rule references it, so no backfill (see the thing_types
// migration for why that rule does not bite here).
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping location_types.metadata_schema")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ location_types.metadata_schema added")
		return nil
	}, func(app core.App) error {
		// Down: no-op. Dropping the column would discard every schema an admin
		// authored, and the field is inert when unused.
		return nil
	})
}
