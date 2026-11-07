package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_membership_timestamps adds `created` and `updated` autodate
// fields to memberships and invites, the only collections without them. The
// console sorts both views on `-created`, and PocketBase rejects an unknown sort
// term while parsing the query, so both views answered 400 for every caller.
//
// The field ids are PocketBase's deterministic defaults (autodate + crc32 of
// the name). A re-import keeps the LIVE field when the name matches and the id
// does not, so an invented id would make this a silent no-op on any install that
// already has the pair.
//
// No backfill, on purpose. Existing rows keep an empty timestamp and sort last
// under `-created` until updated. Nothing records when they were created
// (PocketBase ids are not time-ordered), and an invented date would be read as
// fact.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping membership timestamps import")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Added created/updated to memberships + invites")
		return nil
	}, nil)
}
