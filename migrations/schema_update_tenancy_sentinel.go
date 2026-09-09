package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_tenancy_sentinel closes a cross-tenant read that the allowlist
// discipline could not see, because it is not a role bug at all.
//
// Every inventory read rule scoped on `organization = @request.auth.
// current_organization`. Both sides are TEXT columns whose zero value is the
// empty string, and in PocketBase an empty string equals an empty string. So a
// record whose organization was blank was readable by any authenticated caller
// whose organization context was also blank -- no role check bypassed, no rule
// mis-written, just two sentinels comparing equal.
//
// Both halves were reachable through ordinary product use, which is what made
// it exploitable rather than theoretical:
//
//   - Blank `organization` on a record: organizations.deleteRule used to permit
//     `owner = @request.auth.id`, and 16 of the 18 relations pointing at
//     organizations are non-cascade AND non-required. PocketBase blanks those
//     rather than deleting the rows (core/record_model.go, via SaveNoValidate),
//     so deleting an organization orphaned every thing, location, type, leaf
//     node, nats_account and nebula_ca with a blank organization.
//   - Blank `current_organization` on a caller: the default for a freshly
//     registered invitee before acceptance, and set deliberately on member
//     removal by hooks/membership_lifecycle.go -- the same file whose comment
//     calls this field "the entire read boundary".
//
// Chained, an org owner deleting their organization exposed that tenant's whole
// inventory, its NATS account record and its Nebula CA certificate to any user
// sitting in the blank-context state, in any other tenant.
//
// Three changes, all rules:
//
//  1. The eight affected read rules (things, locations, thing_types,
//     location_types, thing_type_operations, leaf_nodes, nats_accounts,
//     nebula_ca) now require a non-blank context AND a membership in the
//     organization being claimed. The membership clause is the load-bearing
//     half: memberships.organization is required AND cascadeDelete, so it can
//     never be blank, which kills the sentinel by construction rather than by a
//     test someone might later "simplify" away. This is the shape every WRITE
//     rule in the file already had -- which is exactly why the writes were
//     never vulnerable and only the reads were.
//
//     The leaf_nodes branches on those rules needed the same treatment for the
//     same reason: leaf_nodes.organization is itself non-cascade and
//     non-required, so an org delete blanks it too, and a leaf node whose
//     organization was deleted would have matched every orphaned record on the
//     platform.
//
//     Note this does NOT make reads role-scoped -- there is still no role
//     branch, so every role in an org still reads the whole inventory, which
//     CLAUDE.md documents as deliberate. It makes them membership-scoped, which
//     is what "org-scoped" was always supposed to mean.
//
//  2. users.createRule no longer accepts `current_organization`. The update
//     rule already froze it to organizations the caller holds a membership in;
//     the create rule guarded only `is_operator` and the invite email match, so
//     an invited registrant could name any organization's id and land inside a
//     tenant they had no membership in -- then read it, because the read rules
//     scope on precisely that field. This branch is anonymous and cannot check
//     membership even in principle (there is no auth record yet), so the fix is
//     to refuse the field: accept-invite fills it in from the invite once the
//     account exists, and pb-tenancy only writes it when empty.
//
//  3. organizations.deleteRule is platform-operator only, matching updateRule.
//     An owner branch on delete was strictly worse than one on update given the
//     orphaning above, and every console route that touches this collection is
//     already operator-gated, so nothing regresses.
//
// No field changes and no data migration. Rules are applied normally by a
// re-import -- it is only field DEFINITIONS that freeze when a name matches and
// an id does not -- so a re-import is sufficient here.
//
// Orphans already in the database are reported, not deleted. After this
// migration they are unreachable through the API, which is the security fix;
// removing them is a judgement call about customer data that belongs to an
// operator with context, not to a migration running unattended at boot.
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
		// Down: no-op, same as the other rule migrations here. Reverting would
		// drag every unrelated rule back to its previous state, and the thing
		// being reverted to is a cross-tenant read.
		return nil
	})
}

// reportOrphanedTenantRecords lists records whose organization is blank.
//
// These are the rows the sentinel match exposed. They are now unreadable
// through the API, but they are still wrong: an orphaned nats_account describes
// an account whose organization is gone, and an orphaned leaf_node still holds
// a credential. An upgrading operator needs to know they exist.
//
// The collection list is derived from the schema rather than hardcoded, so a
// collection added later is covered without anyone remembering to add it here.
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
