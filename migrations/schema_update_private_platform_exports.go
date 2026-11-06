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
// Both were public (token_req false). A public export can be imported by any
// account, and an owner may create an import naming any account key, so:
//
//   - the per-org inventory export on the provider account (inventory-<org id>)
//     was readable by every tenant that knew the hub key, which its own import
//     record shows, and the victim's code, which is printed on labels;
//   - a managed tenant's helpdesk-events export was readable by any tenant that
//     knew that tenant's account key.
//
// The fix is pb-nats v0.3.3: an export lists the accounts it grants in
// allowed_importers, and pb-nats signs the activation token for those alone. This
// migration declares that field and revocations with PocketBase's deterministic
// ids (pb-nats also creates them, see migrations/CLAUDE.md), then re-saves every
// managed organization, because ensureManagedExports reconciles only when an
// organization is saved. The inventory pairs need no re-save: the serve-time
// sweep reconciles every organization's pair.
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
			// Fires syncManaged, whose reconcile logs the fields it changed.
			// Failures there are logged, not returned, as in
			// schema_update_subject_token_to_code.go.
			if err := app.Save(org); err != nil {
				return fmt.Errorf("re-save organization %q: %w", org.GetString("name"), err)
			}
		}

		log.Printf("✅ export grant fields added; %d managed organization export(s) made private to the hub", len(orgs))
		return nil
	}, nil)
}
