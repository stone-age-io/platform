package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_membership_authority makes memberships.role the only source of
// authority over an organization's members and invitations. Two rule changes on
// memberships and invites:
//
//  1. The organizations.owner branches are gone. They never checked the owner's
//     membership, so a demoted or removed owner kept managing members and could
//     PATCH their own role back to owner.
//  2. memberships.user is frozen on every branch, not only the self branch.
//     Otherwise a removed owner could PATCH any membership to
//     {"user":"<self>","role":"owner"} and be back in.
//
// Rule-only, so a re-import is enough. It reports owners with no owner/admin
// membership rather than granting one: those owners now have no access, and
// whether they should is an operator decision.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping membership authority update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ memberships.role is now the only authority over members and invitations; memberships.user is frozen")

		reportOwnersWithoutMembership(app)
		return nil
	}, nil)
}

func reportOwnersWithoutMembership(app core.App) {
	orgs, err := app.FindAllRecords("organizations")
	if err != nil {
		log.Printf("⚠️ Could not list organizations to check owner memberships: %v", err)
		return
	}
	for _, org := range orgs {
		owner := org.GetString("owner")
		if owner == "" {
			continue
		}
		_, err := app.FindFirstRecordByFilter("memberships",
			"user = {:u} && organization = {:o} && (role = 'owner' || role = 'admin')",
			dbx.Params{"u": owner, "o": org.Id})
		if err != nil {
			log.Printf("⚠️ Organization %q (%s): its owner %s has no owner or admin membership, "+
				"so no longer has any access to it. If they should, invite them as admin "+
				"(an operator can invite into any organization).",
				org.GetString("name"), org.Id, owner)
		}
	}
}
