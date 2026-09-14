package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_email_templates adds the collection that makes the platform's
// own outgoing mail editable without a rebuild.
//
// PocketBase's editable mail templates are a CLOSED set of five, all of them
// record flows on an auth collection: verification, password reset, confirm
// email change, OTP, and the new-device auth alert. There is no way, in /_ or in
// the API, to register a sixth. So the organization invitation -- which the
// platform composes itself -- had nowhere to live but a Go const in pb-tenancy,
// and changing one word of it meant editing a library, tagging it, `go get`,
// rebuild, redeploy. For copy an operator is better placed to write than a
// developer.
//
// Superuser-only, by all five rules being null. An email body is close enough to
// code -- it is a Go template rendered into HTML and mailed under the
// deployment's own sender identity -- that a tenant owner has no business
// editing one, and there is nothing tenant-scoped about it to scope.
//
// NO ROWS ARE SEEDED HERE. hooks.RegisterEmailTemplates creates the missing ones
// at serve time from the compiled-in values, which keeps the Go value the single
// source of the default, lets a template added in a later release appear without
// another migration, and -- because it only ever creates what is absent -- can
// promise never to overwrite an operator's edit. A re-running schema import
// could not promise that.
//
// Import-only otherwise, same shape as schema_update_viewer_role.go. Nothing
// existing is touched, so there is nothing to back-fill and nothing to revert
// that dropping the collection would not do better.
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
