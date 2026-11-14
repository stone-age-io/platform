package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_activity adds the `activity` collection: a tenant-facing feed of
// who changed what in the console.
//
// It is not a read branch on audit_logs: that has no `organization` column, and
// `auditSnapshotCollections` in main.go stores full record values in it.
// An entry must be visible to exactly those who could read the record it
// describes, so the feed covers only the collections whose reads are org-scoped
// with no role branch. The list lives in hooks/activity.go, pinned by a test.
//
//   - `organization` is required and cascade, so it can never hold the blank
//     that makes a blank-to-blank comparison true (see
//     schema_update_tenancy_sentinel.go).
//   - `actor` is text, not a relation: deleting a user would blank a relation
//     row by row, and text can also hold a _superusers id.
//
// Field ids use PocketBase's deterministic form (see the re-import note in
// migrations/CLAUDE.md). Import-only; nothing to backfill.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			return errNoSchema
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ activity added: a tenant-facing feed of who changed what in the console")
		return nil
	}, nil)
}
