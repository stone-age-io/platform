package migrations

import (
	"log"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_membership_authority makes memberships.role the only source of
// authority over an organization's members and invitations.
//
// Two rule changes, both on the memberships and invites collections:
//
//  1. The organizations.owner branches are gone. They checked the owner relation
//     and never the owner's membership, so the field was a standing grant: an
//     owner whom an admin demoted to dashboard, or removed outright, still
//     listed and managed invitations and memberships, and could PATCH their own
//     role back to owner. owner and admin were meant to be identical in the
//     rules (CLAUDE.md), and the membership already says who is which.
//  2. memberships.user is frozen on every branch. Only the self branch froze it,
//     so combined with (1) a removed owner could PATCH any membership id in the
//     organization to {"user":"<self>","role":"owner"} and be back in,
//     displacing the member who held it.
//
// Rule-only, so a re-import is sufficient. What it cannot do is tell an owner
// who was deliberately removed from one who simply never had a membership, so
// it reports organizations whose owner holds no owner/admin membership rather
// than granting one: from this release those owners have no access, and
// whether they should is an operator decision, not a migration one.
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
