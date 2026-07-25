package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_device_active_flag adds the `active` flag to things and
// leaf_nodes, sets both collections' authRule to `active = true`, and gives
// things the manageRule it was missing.
//
// The backfill is the load-bearing part. `active` is a new bool column, so every
// existing row gets SQLite's zero value — false. Importing the new authRule
// without the UPDATE below would lock every already-provisioned device and edge
// box out of the API on the next deploy. Order matters: backfill first, import
// second, so there is no window in which the rule is live over unbacked rows.
//
// Also note the migration writes the column directly rather than iterating
// records through app.Save(). That deliberately bypasses hooks/active_flag.go —
// a save would fire the update hook, and while the was==now guard makes it a
// no-op here, going through the ORM to set a column that every row must have is
// slower and less obvious than saying so in SQL.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping device active flag update")
			return nil
		}

		// Backfill before the rule goes live. The columns may not exist yet on a
		// fresh database (the import below creates them), so a failure here is
		// expected in that case and is not fatal — there are no rows to strand.
		for _, table := range []string{"things", "leaf_nodes"} {
			res, err := app.DB().NewQuery(
				"UPDATE {{" + table + "}} SET [[active]] = true WHERE [[active]] IS NULL OR [[active]] = false",
			).Execute()
			if err != nil {
				log.Printf("ℹ️ Skipped %s active backfill (column not present yet): %v", table, err)
				continue
			}
			if n, err := res.RowsAffected(); err == nil && n > 0 {
				log.Printf("✅ Backfilled active=true on %d existing %s record(s)", n, table)
			}
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		// Fresh databases created the columns during the import above, so run the
		// backfill once more to cover rows the import may have brought along.
		for _, table := range []string{"things", "leaf_nodes"} {
			if _, err := app.DB().NewQuery(
				"UPDATE {{" + table + "}} SET [[active]] = true WHERE [[active]] IS NULL OR [[active]] = false",
			).Execute(); err != nil {
				return err
			}
		}

		log.Println("✅ things/leaf_nodes active flag applied (authRule active = true, things manageRule added)")
		return nil
	}, nil)
}
