package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_dashboard_role renames the `badge` membership/invite role to
// `dashboard`.
//
// The role is the restricted login that gets the Visualizer and nothing else;
// it was named for a removed badge feature. ("kiosk" belongs to another project.)
//
// Import first: it changes the `role` select options, and an import does not
// revalidate existing rows. Then a raw UPDATE rewrites them, because app.Save()
// would fire the membership hooks (re-provisioning, mail) for a string rename.
//
// The rename cannot widen access: every rule is an allowlist, so a row left
// holding `badge` fails closed. The UPDATE runs so those logins keep working.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			return errNoSchema
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		// memberships holds the live role; invites the pending one.
		for _, table := range []string{"memberships", "invites"} {
			res, err := app.DB().NewQuery(
				"UPDATE {{" + table + "}} SET [[role]] = 'dashboard' WHERE [[role]] = 'badge'",
			).Execute()
			if err != nil {
				return err
			}
			if n, err := res.RowsAffected(); err == nil && n > 0 {
				log.Printf("✅ Renamed badge → dashboard on %d %s row(s)", n, table)
			}
		}

		log.Println("✅ badge role renamed to dashboard (memberships + invites)")
		return nil
	}, nil)
}
