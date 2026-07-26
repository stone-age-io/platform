package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_thing_type_metadata_schema adds `thing_types.metadata_schema`,
// an optional JSON Schema describing the inventory fields tracked for a class
// of device (service dates, asset tags, warranty references).
//
// This is the registry half of the metadata work. `things.metadata` has always
// accepted arbitrary JSON; what it lacked was any statement of what a given
// type is SUPPOSED to carry, so every Thing's metadata was hand-authored and
// nothing was consistent across two devices of the same kind. A schema here
// lets the Thing form render typed inputs (ui/src/components/common/
// MetadataEditor.vue) instead of a textarea.
//
// No backfill is needed and none is included. The rule in CLAUDE.md — a new
// non-null column with a live rule over it must be backfilled in the same
// migration, or existing rows land as the zero value and fail the rule — does
// not bite here on either count: the field is nullable, and no API rule
// references it. A type with no schema is the normal state, not a broken one;
// MetadataEditor treats absent, empty, and property-less schemas identically
// and falls back to free-form key/value rows.
//
// Nor does this constrain what may be written. The schema drives form
// RENDERING only — `things.metadata` stays free-form JSON and no rule or hook
// validates a Thing's metadata against its type's schema. That is deliberate:
// validation would make editing a type's schema retroactively invalidate
// existing Things, and the write path (POST /api/org/things and the CRUD
// update rule) would have to reject saves for a reason the member did not
// cause and cannot fix. Extra keys are surfaced in the form and preserved on
// save.
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
		// Down: no-op. Dropping the column would discard every schema an admin
		// authored, and the field is inert when unused.
		return nil
	})
}
