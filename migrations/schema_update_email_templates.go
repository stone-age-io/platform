package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_email_templates adds the collection that makes the platform's
// own outgoing mail editable without a rebuild.
//
// PocketBase's editable templates are a closed set of auth-collection flows, so
// the organization invitation, which the platform composes itself, had to be
// compiled in and needed a rebuild to change one word.
//
// Superuser-only (all five rules null): a body is a Go template mailed under the
// deployment's own sender identity, and nothing about it is tenant-scoped.
//
// No rows are seeded here. hooks.RegisterEmailTemplates creates missing ones at
// serve time from the compiled-in defaults, and since it only creates what is
// absent it never overwrites an operator's edit. Import-only.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping email_templates import")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ email_templates added: the invitation email is now editable in /_ (Collections → email_templates)")
		return nil
	}, nil)
}
