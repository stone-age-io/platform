package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_identity_link_freeze closes two rule gaps, both rules only.
//
//  1. memberships.updateRule, the self branch, let any role choose WHICH NATS
//     identity its own membership links to. nats_users.viewRule branch 2 then
//     serves the creds_file of whatever identity that field names, so a
//     `dashboard` login could PATCH its membership to point at a gateway, or at
//     the owner, and read that credential -- NKEY seed included. Every nats_users
//     id in the organization is discoverable by any role through things.nats_user.
//     hooks/relation_tenancy.go never caught it because the target was in the
//     SAME organization. The self branch may now only keep or clear the link;
//     choosing one is owner/admin, as it already was through MemberDetailView.
//     The Settings page picker is unaffected in practice: a member can only
//     list the one identity already linked to them.
//
//  2. users.createRule let a platform operator create a user with is_operator
//     set, contradicting users.updateRule and every statement that the API
//     cannot grant it. The operator branch now refuses the field.
//
// No field changes, so a re-import is sufficient: collection rules are applied
// normally, only field definitions freeze when a name matches and an id does not.
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
		// Down: no-op, same as the other rule migrations. Reverting restores a
		// credential read across roles.
		return nil
	})
}
