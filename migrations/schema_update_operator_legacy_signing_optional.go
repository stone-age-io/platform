package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_operator_legacy_signing_optional drops the required flag from
// nats_system_operator's signing_public_key / signing_private_key / signing_seed.
//
// These three are fossils. pb-nats used to keep one operator signing key in scalar
// fields; it now keeps a list in signing_keys / signing_keys_private and migrates the
// old values across (ensureOperatorSigningKeysFields). It no longer writes the scalars,
// and its own collection definition does not declare them at all — they exist here only
// because schema.json is a dump taken while they still did.
//
// Required-but-never-written is a latent trap rather than a cosmetic wart, because
// PocketBase validates the whole record on every save. pb-nats creates the operator
// record before this schema is imported, so on a fresh install those three columns
// arrive empty and required — and the next save of that record, whatever writes it and
// for whatever reason, fails with "signing_private_key: cannot be blank". Nothing
// re-saves the operator during normal operation, which is why this has gone unnoticed;
// the paths that do are all repairs (resolving the system account, regenerating the
// operator JWT), so the trap is armed precisely for the moment something has already
// gone wrong.
//
// Worse, it is unrecoverable in place. pb-nats initializes from OnBootstrap, which fires
// for every command, so an operator save it cannot complete takes down `migrate up` as
// well as `serve` — the binary cannot reach the migration that would relax the flag.
// Verified by reproducing it: a database in that state refuses every command until the
// column is edited out of band.
//
// No backfill. Relaxing a constraint cannot strand an existing row, and the fields stay
// in place with whatever values they already hold so ensureOperatorSigningKeysFields can
// still read them on an old database that has not yet migrated its keys.
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
