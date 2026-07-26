package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_dashboard_role renames the `badge` membership/invite role to
// `dashboard`.
//
// The role never had anything to do with badges: it is the restricted console
// login that gets the Visualizer and nothing else. It was named for a badge
// feature that was removed (the identity card read a `badges` KV bucket nothing
// ever wrote), and "kiosk" was rejected because that name belongs to a separate
// project.
//
// Order is import-then-rewrite, and the rewrite is raw SQL on purpose:
//
//  1. The import widens each `role` select to the new option set. Existing rows
//     still holding `badge` are not revalidated by an import, so they survive
//     the window between the two steps.
//  2. The UPDATE rewrites them. It bypasses the ORM deliberately — going through
//     app.Save() would fire pb-tenancy's membership hooks (re-provisioning, mail)
//     for what is a pure rename of a stored string.
//
// This rename cannot widen access. Every rule in schema.json is an ALLOWLIST
// naming owner/admin/member explicitly, so an option disappearing from the
// select grants nothing — which is the whole reason the deny-list lesson in
// CLAUDE.md exists. A row left holding `badge` would fail every allowlist and be
// locked out of writes rather than escalated, so the failure mode here is closed,
// not open. The UPDATE still runs so those logins keep working.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping dashboard role rename")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		// memberships is the live role; invites carries the pending one. A fresh
		// database has no rows to rewrite and reports zero.
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
