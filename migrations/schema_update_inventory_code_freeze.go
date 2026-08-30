package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_inventory_code_freeze adds `@request.body.code:changed = false`
// to the update rules of things, locations, thing_types and location_types --
// the four code-scoped collections that were not already frozen.
//
// leaf_nodes has carried this term since schema_update_unique_org_code.go, with
// the reason written beside it: the code is a KV key prefix and a JetStream
// domain suffix, so changing it silently orphans everything the edge already
// wrote. The other four have the same exposure and never got the same guard.
// A code is what a sibling app resolves (organization, code) against, what a
// seeded or mirrored catalog matches on, and -- under ADR 0002 -- what may be
// printed on a label screwed to the device. All three break silently on a
// rename: nothing errors, the lookups simply stop finding anything.
//
// Rules only. No field changes, no data migration, and deliberately no sweep:
// the UNIQUE (organization, code) indexes these collections need were already
// added by schema_update_unique_org_code.go, and freezing a column does not
// invalidate any value already in it.
//
// This does not close the hole, and is not meant to. An update rule guards the
// record API; a superuser in the PocketBase dashboard bypasses it, as does any
// server-side app.Save. That is the accepted sharp edge in ADR 0002 -- the term
// stops the routine accident, not the deliberate act.
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
		// Down: no-op. Un-freezing is a rule edit an operator can make in the
		// dashboard, and reverting the schema wholesale would drag every other
		// rule back with it.
		return nil
	})
}
