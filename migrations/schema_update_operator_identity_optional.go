package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_operator_identity_optional drops the required flag from
// nats_system_operator's private_key and seed.
//
// These two are the operator's identity key material. Since pb-nats moved the
// $SYS.REQ.CLAIMS.DELETE claim onto the operator's signing key, nothing in a running
// deployment reads either one — account JWTs and account withdrawals are both signed
// with LatestSigningKey(), and the identity key signs only the operator JWT itself,
// which is regenerated solely on a repair path. They stay declared and every deployment
// still populates them; they are simply no longer load-bearing.
//
// This has to be done here as well as in pb-nats, and that is the whole point of the
// migration. nats_system_operator is declared twice — pb-nats builds it in
// createSystemOperatorCollection, and schema.json carries a dump of it — and on a
// collection that already exists the import wins. ImportCollections unmarshals the
// imported `fields` array onto the existing collection, and only re-adds existing fields
// whose id is *absent* from the import (core/collection_import.go); schema.json carries
// real field ids, so `seed` and `private_key` are present and their imported definition
// stands. Relaxing the flag in pb-nats alone would therefore be undone by the next
// schema re-import, silently and durably.
//
// The hazard being removed is required-and-blank. PocketBase validates the whole record
// on every save, so a blank required column fails the *next* write to the operator
// record for an unrelated reason. And because pb-nats initializes from OnBootstrap,
// which fires for every command, an operator save it cannot complete takes down
// `migrate up` alongside `serve` — there is then no in-band way to reach the migration
// that would relax the flag, and the column has to be edited out of band. This is the
// same trap schema_update_operator_legacy_signing_optional cleared for the fossil
// signing key columns; it was verified there by reproducing it.
//
// No backfill: relaxing a constraint cannot strand an existing row, and nothing clears
// these fields today. This only keeps the door open.
//
// Additive import (deleteMissing=false); safe on fresh DBs.
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
