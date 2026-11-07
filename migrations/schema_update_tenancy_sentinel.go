package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_tenancy_sentinel closes a cross-tenant read that is not a role
// bug at all.
//
// Every inventory read rule scoped on `organization = @request.auth.
// current_organization`. Both sides are TEXT whose zero value is the empty
// string, and in PocketBase an empty string equals an empty string. So a record
// with a blank organization was readable by any caller whose own context was
// also blank. Both states were ordinary product use:
//
//   - Blank `organization`: an owner could delete their organization, and
//     PocketBase blanks rather than deletes rows on the non-cascade,
//     non-required relations into it (16 of 18), orphaning its things,
//     locations, types, leaf nodes, nats_account and nebula_ca.
//   - Blank `current_organization`: a registered invitee before acceptance, and
//     a removed member (hooks/membership_lifecycle.go).
//
// Chained, deleting an organization exposed its inventory, NATS account record
// and Nebula CA certificate to blank-context users in every other tenant.
//
// Three rule changes:
//
//  1. The eight affected read rules (things, locations, thing_types,
//     location_types, thing_type_operations, leaf_nodes, nats_accounts,
//     nebula_ca) require a non-blank context AND a membership in that
//     organization. The membership clause is the load-bearing half:
//     memberships.organization is required and cascade, so it is never blank.
//     Every write rule already had it, which is why only reads were exposed.
//     The leaf_nodes branches got the same fix, since an org delete blanks
//     leaf_nodes.organization too. Reads stay org-scoped, not role-scoped.
//  2. users.createRule refuses `current_organization`. An invited registrant
//     could name any organization's id and read it, and an anonymous create
//     cannot check membership. Accept-invite fills it in, and hooks/invites.go
//     writes it only when empty.
//  3. organizations.deleteRule is platform-operator only, matching updateRule.
//
// Rules re-import normally (only field definitions freeze), so a re-import is
// enough. Existing orphans are reported, not deleted: the API can no longer
// reach them, and removing customer data is an operator's call.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping tenancy sentinel update")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ Tenancy sentinel closed: blank organization context no longer scopes a read; current_organization is not settable at registration; deleting an organization is operator-only")

		reportOrphanedTenantRecords(app)

		return nil
	}, func(app core.App) error {
		// No-op: reverting would restore a cross-tenant read.
		return nil
	})
}

// reportOrphanedTenantRecords lists records whose organization is blank. They
// are unreadable now but still wrong (an orphaned leaf_node holds a
// credential), so the operator needs to know. Collections are found from the
// schema, so later ones are covered.
func reportOrphanedTenantRecords(app core.App) {
	collections, err := app.FindAllCollections()
	if err != nil {
		log.Printf("⚠️ Could not enumerate collections to check for orphaned records: %v", err)
		return
	}

	total := 0
	for _, col := range collections {
		field := col.Fields.GetByName("organization")
		if field == nil || field.Type() != "relation" {
			continue
		}

		orphans, err := app.FindRecordsByFilter(col.Name, "organization = ''", "", 0, 0)
		if err != nil {
			log.Printf("⚠️ Could not check %s for orphaned records: %v", col.Name, err)
			continue
		}
		if len(orphans) == 0 {
			continue
		}

		total += len(orphans)
		log.Printf("🔎 %s: %d record(s) with a blank organization", col.Name, len(orphans))
	}

	if total > 0 {
		log.Printf("⚠️ %d orphaned record(s) found, left in place. Until this migration these were readable by any caller whose own organization context was blank. They are now unreachable through the API. They were almost certainly produced by deleting an organization, which used to blank rather than cascade; review and remove them deliberately.", total)
	}
}
