package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_membership_rules re-imports the corrected memberships
// create/update/delete rules. The old rules used @collection.memberships.X with
// no row correlation, so admins acting on other members got 404. The
// back-relation pattern (organization.memberships_via_organization.X, and
// @request.auth.memberships_via_user.X for create) makes the user and role
// conditions match the same membership row.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping membership rules update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Updated memberships create/update/delete rules to use back-relation pattern")
		return nil
	}, nil)
}
