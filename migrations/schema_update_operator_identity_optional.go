package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_operator_identity_optional drops the required flag from
// nats_system_operator's private_key and seed. Nothing in a running deployment
// reads them now: account JWTs and withdrawals are signed with
// LatestSigningKey(), and the identity key signs only the operator JWT, on a
// repair path. They are still populated.
//
// It must be done here as well as in pb-nats: schema.json also declares the
// collection with real field ids, so on an existing collection the imported
// definition wins and would undo a pb-nats-only change.
//
// Required-and-blank is the hazard. PocketBase validates the whole record on
// every save, and pb-nats saves the operator from OnBootstrap, which runs for
// every command, so a failed save also blocks `migrate up` and the column has to
// be fixed out of band. Same trap as
// schema_update_operator_legacy_signing_optional. No backfill needed.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping operator identity field update")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ nats_system_operator identity fields (private_key, seed) are optional")
		return nil
	}, nil)
}
