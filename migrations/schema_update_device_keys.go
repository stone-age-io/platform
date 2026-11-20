package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_device_keys declares device-held keys (plans/device-held-keys.md):
// `device_key` on nats_users and nebula_hosts, `public_key` on nebula_hosts, and
// the unique index on nats_users.public_key.
//
// An import only, with no rule change and no backfill:
//
//   - pb-nats and pb-nebula create these on boot too, so the field ids are
//     PocketBase's deterministic ones (type + crc32 of the name). A re-import
//     keeps the live field when the name matches and the id does not
//     (migrations/CLAUDE.md), so any other id would make this a silent no-op.
//   - Existing rows land with device_key = false, which is the behaviour they
//     already have. Nothing to backfill.
//   - No rule term. The existing owner/admin update rules already say who may
//     paste a key. What a write may do (device_key frozen, public_key
//     write-once and only on a device row) is enforced by the libraries, for
//     every write path, not only REST.
//   - The index is listed here because a re-import replaces a collection's
//     indexes with the imported list. pb-nats would put it back on the next
//     boot, but not before the boot that ran the import.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			return errNoSchema
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ device-held key fields on nats_users and nebula_hosts")
		return nil
	}, nil)
}
