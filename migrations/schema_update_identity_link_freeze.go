package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_identity_link_freeze closes two rule gaps, both rules only.
//
//  1. memberships.updateRule, self branch, let any role choose which NATS
//     identity its membership links to, and nats_users.viewRule serves the
//     creds_file of that identity. So a `dashboard` login could point it at a
//     gateway or the owner and read the NKEY seed (ids are discoverable through
//     things.nats_user, and relation_tenancy.go allows same-org targets). The
//     self branch may now only keep or clear the link; choosing is owner/admin.
//  2. users.createRule let an operator create a user with is_operator set,
//     which the API must never grant. The operator branch now refuses it.
//
// Rules only, so a re-import applies them.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping identity link freeze")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ Identity link frozen: a member may keep or clear its own NATS identity link but not re-point it; operators cannot mint operators over REST")
		return nil
	}, func(app core.App) error {
		// Down: no-op. Reverting would restore a credential read across roles.
		return nil
	})
}
