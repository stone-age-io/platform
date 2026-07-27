package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_viewer_role adds a `viewer` option to the membership and invite
// role selects: a tenant's read-only staff.
//
// This migration is an import and nothing else — no rule text changes, and no
// backfill. Both are worth stating, because both are the kind of omission that
// usually means a migration is wrong:
//
//   - No rule changes, because every write branch in schema.json is an ALLOWLIST
//     that names the roles it admits (see CLAUDE.md). A role value that appears
//     in no allowlist is denied everywhere by construction. That is the whole
//     dividend of the allowlist refactor: a read-only role costs one enum entry
//     on the server. If you find yourself editing a rule to keep `viewer` out,
//     that rule is a deny-list and it is the bug.
//   - No backfill, because no row can already hold `viewer`. Contrast
//     schema_update_device_active_flag.go, where a new non-null column with a
//     live rule over it stranded every existing row until the UPDATE ran. Adding
//     an option to a select strands nothing.
//
// `viewer` reads exactly what `member` reads — the read rules on things,
// locations, thing_types, location_types, message_schemas and leaf_nodes are
// org-scoped with no role check, and deliberately stay that way. The distinction
// between the two roles is writes, which the allowlists already handle, plus
// console navigation, which is ui/src/stores/auth.ts.
//
// `dashboard` is untouched and remains the zero-authority probe in
// scripts/test-authz.sh. `viewer` cannot replace it: it holds read capability,
// so a denial it passes proves less than a denial `dashboard` passes.
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
