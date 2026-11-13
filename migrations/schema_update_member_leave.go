package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_member_leave lets a user delete their own membership, except
// an owner: memberships.deleteRule gains a second branch that names every
// other role.
//
// Before this, only an owner or admin could delete a membership. The console
// showed Leave Org to member, viewer and dashboard as well, and the delete
// always failed with a 404. The owner stays out of the new branch, matching the
// console, which never offers an owner the button.
//
// An import only: a rule change, no field change and nothing to backfill.
// scripts/test-authz.sh section 28 checks both halves.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping member leave import")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ memberships: a non-owner can leave an organization")
		return nil
	}, nil)
}
