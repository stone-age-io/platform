package migrations

import (
	"fmt"
	"log"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_private_platform_exports makes the two exports the platform
// creates for itself private to the one account each is meant for.
//
// Both were public (token_req false), and any account may import a public
// export. So any tenant could read another's inventory export
// (inventory-<org id>) given the hub key and the victim's code, and a managed
// tenant's helpdesk-events export given its account key.
//
// The fix is pb-nats v0.3.3: pb-nats signs activation tokens only for the
// accounts in an export's allowed_importers. This declares that field and
// revocations with PocketBase's deterministic ids (migrations/CLAUDE.md), then
// re-saves every managed organization, because ensureManagedExports reconciles
// only on save. The serve-time sweep reconciles the inventory pairs.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping private platform exports import")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		orgs, err := app.FindAllRecords("organizations", dbx.HashExp{"managed": true})
		if err != nil {
			return fmt.Errorf("find managed organizations: %w", err)
		}
		for _, org := range orgs {
			// Fires syncManaged. Its reconcile failures are logged, not
			// returned, as in schema_update_subject_token_to_code.go.
			if err := app.Save(org); err != nil {
				return fmt.Errorf("re-save organization %q: %w", org.GetString("name"), err)
			}
		}

		log.Printf("✅ export grant fields added; %d managed organization export(s) made private to the hub", len(orgs))
		return nil
	}, nil)
}
