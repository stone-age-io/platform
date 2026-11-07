package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_device_auth_alert turns off PocketBase's new-device login alert
// on the auth collections whose records are devices rather than people.
//
// `things` and `leaf_nodes` had the PocketBase default, on. Their addresses are
// synthetic (hooks/thing_routes.go mints `<code>@<org>.thing.local`), and the
// send blocks: PocketBase's authAlert() waits up to 15 seconds for the mail
// before returning the auth response, on every auth from a new origin
// (MD5 of ip + user-agent). A device on a changing address pays that repeatedly.
//
// Left on for `users` and `_superusers`: people with real addresses, who can act
// on the alert. Import-only; the setting is a collection option, not a column.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping device auth alert update")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ New-device login alerts disabled on device auth collections (undeliverable synthetic addresses; the send blocks auth for up to 15s)")
		return nil
	}, nil)
}
