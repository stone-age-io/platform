package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_membership_rules re-imports schema.json so existing deployments
// pick up the corrected memberships create/update/delete rules. The previous
// rules used @collection.memberships.X with no row correlation (and "=" on role)
// which caused requests to return 404 for admins acting on other members.
// Switched to the back-relation pattern (organization.memberships_via_organization.X
// for view/update/delete; @request.auth.memberships_via_user.X for create) so the
// user/role conditions correlate to the SAME membership row. Additive import —
// safe on fresh DBs.
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
