package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_inventory_code_freeze adds `@request.body.code:changed = false`
// to the update rules of things, locations, thing_types and location_types --
// the four code-scoped collections that were not already frozen.
//
// leaf_nodes already had it. A code is what sibling apps resolve
// (organization, code) against, what catalogs match on, and (ADR 0002) what is
// printed on the device label; a rename breaks all of them silently.
//
// Rules only; freezing a column invalidates no existing value. A superuser in the
// dashboard or a server-side app.Save still bypasses it, the accepted sharp edge
// in ADR 0002: the term stops the accident, not the deliberate act.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping inventory code freeze")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ code frozen on things, locations, thing_types, location_types")
		return nil
	}, func(app core.App) error {
		// Down: no-op. Reverting would re-import every other rule too.
		return nil
	})
}
