package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_operator_legacy_signing_optional drops the required flag from
// nats_system_operator's signing_public_key / signing_private_key / signing_seed.
//
// pb-nats no longer writes these scalars; it keeps a list in signing_keys /
// signing_keys_private and migrates the old values across
// (ensureOperatorSigningKeysFields). They exist only in schema.json.
//
// Required-but-never-written is a trap because PocketBase validates the whole
// record on every save. On a fresh install the columns arrive empty, so the next
// operator save (only repair paths do one) fails with "signing_private_key:
// cannot be blank". pb-nats saves from OnBootstrap, which runs for every
// command, so that also blocks `migrate up` until the column is fixed out of
// band.
//
// No backfill. The fields keep their values so ensureOperatorSigningKeysFields
// can still read them on a database that has not migrated its keys.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping operator legacy signing field update")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ nats_system_operator legacy signing fields are optional")
		return nil
	}, nil)
}
