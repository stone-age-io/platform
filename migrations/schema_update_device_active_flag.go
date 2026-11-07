package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_device_active_flag adds the `active` flag to things, sets the
// authRule to `active = true`, and gives things the manageRule it was missing.
//
// It once did the same for leaf_nodes. That half was removed with the collection
// (schema_update_drop_leaf_nodes.go), because the fatal second pass below would
// fail on a fresh database with no such table. The migration name stays, since
// the _migrations ledger and the schema_version readiness check key on it.
//
// The backfill is load-bearing: a new bool column lands as false, so importing
// the authRule without the UPDATE would lock every provisioned device out of the
// API on deploy. It runs before the import so the rule is never live over
// unbacked rows. Plain SQL, not app.Save(): hooks/active_flag.go would have
// nothing to do.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping device active flag update")
			return nil
		}

		// On a fresh database the column does not exist yet, so a failure here is
		// expected and not fatal: there are no rows to strand.
		res, err := app.DB().NewQuery("UPDATE {{things}} SET [[active]] = true WHERE [[active]] IS NULL OR [[active]] = false").Execute()
		if err != nil {
			log.Printf("ℹ️ Skipped the things active backfill (column not present yet): %v", err)
		} else if n, err := res.RowsAffected(); err == nil && n > 0 {
			log.Printf("✅ Backfilled active=true on %d existing things record(s)", n)
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		// Second pass, now that the import has created the column.
		if _, err := app.DB().NewQuery("UPDATE {{things}} SET [[active]] = true WHERE [[active]] IS NULL OR [[active]] = false").Execute(); err != nil {
			return err
		}

		log.Println("✅ things active flag applied (authRule active = true, things manageRule added)")
		return nil
	}, nil)
}
