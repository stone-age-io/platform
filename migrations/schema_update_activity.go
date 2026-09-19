package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_activity adds the `activity` collection: a tenant-facing feed of
// who changed what in the console, and when.
//
// WHY IT IS NOT audit_logs. The forensic trail is operator-only and stays that
// way. It has no `organization` column to scope on, its rows are shaped for
// forensics, and `auditSnapshotCollections` in main.go puts full before/after
// VALUES for several collections into it -- `nats_roles` values are
// publish/subscribe permission sets. Widening a read on audit_logs would inherit
// the exposure of every collection on that list at once. A separate, narrow,
// names-nothing collection is the smaller and safer thing.
//
// WHAT IT COVERS, AND THE INVARIANT THAT DECIDES IT. An activity entry is
// visible to exactly those who could read the record it describes. A flat
// collection mirroring other collections inherits none of their scoping, which
// is what audit_logs taught expensively. So the feed covers only the five
// tenant collections whose own read rules are org-scoped with no role branch --
// things, locations, thing_types, location_types, thing_type_operations -- and
// not memberships, invites, nats_roles or nebula_networks, whose reads stop at
// owner/admin. The list lives in hooks/activity.go and is pinned by a test.
//
// TWO SCHEMA DECISIONS WORTH THE INK:
//
//   - `organization` is required AND cascade, unlike locations where it is
//     optional and non-cascade. That is schema_update_tenancy_sentinel.go
//     applied at design time: a required cascade column can never hold ”, so
//     the ” = ” read collapse is impossible by construction rather than by a
//     comparison someone may later tidy away.
//   - `actor` is TEXT, not a relation to users. A non-cascade relation is
//     BLANKED when its target is deleted -- one SaveNoValidate per row
//     (core/record_model.go), a write storm proportional to the feed -- and a
//     blank relation is another sentinel to reason about. Text also holds a
//     _superusers id, which a relation into users cannot, and superusers reach
//     these collections through the same record endpoints. `expand` is no loss:
//     users.viewRule would return nothing for most readers anyway.
//
// Field ids are PocketBase's deterministic form (<type> + crc32 of the name).
// Nothing else creates this collection, so they are used verbatim rather than
// reconciled -- but they follow the convention so that a future library or a
// re-import cannot silently keep a different definition. See the re-import note
// in CLAUDE.md.
//
// Import-only. No rows are seeded and nothing existing is touched, so there is
// nothing to back-fill and nothing a down-migration could do that dropping the
// collection would not do better.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping activity import")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ activity added: a tenant-facing feed of who changed what in the console")
		return nil
	}, nil)
}
