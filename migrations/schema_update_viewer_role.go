package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_viewer_role adds a `viewer` option to the membership and invite
// role selects: a tenant's read-only staff.
//
// An import only, with no rule changes and no backfill:
//
//   - Every write branch is an allowlist naming its roles (AUTHORIZATION.md), so
//     a role in no allowlist is denied everywhere by construction. If a rule
//     needs editing to keep `viewer` out, that rule is a deny-list and is the bug.
//   - No row can already hold `viewer`, so nothing needs a backfill.
//
// `viewer` reads what `member` reads, since read rules are org-scoped with no
// role check. The difference is writes, plus console navigation in
// ui/src/stores/auth.ts.
//
// `dashboard` stays the zero-authority probe in scripts/test-authz.sh; `viewer`
// holds read capability, so a denial it passes proves less.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping viewer role import")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ viewer role added to memberships + invites")
		return nil
	}, nil)
}
