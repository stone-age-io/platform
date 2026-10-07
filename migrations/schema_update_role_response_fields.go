package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_role_response_fields declares nats_roles.allow_response,
// allow_response_max and allow_response_ttl, which schema.json never listed.
//
// Whether a database HAS them depends on when it was created. pb-nats creates
// nats_roles before schema.json is imported (superuser upsert runs first), and a
// re-import never removes a field, so a database created after pb-nats added
// these in January 2026 got them from the library. One created before that --
// this platform's first commit is December 2025 -- never did: pb-nats added role
// fields only when it created the collection, and schema.json did not declare
// them. PocketBase discards a write to an undeclared field without an error, so
// on such a database the console's "Response Permissions" toggle saved,
// reported success and stored nothing, and every role read allow_response =
// false.
//
// pb-nats v0.3.0 adds any missing role field at startup, so a running server is
// already fixed by the dependency bump. This migration declares them anyway, so
// the schema says what the collection holds and a database brought up with
// `migrate up` does not depend on a library's startup pass having run.
// role_response_fields_test.go builds the legacy shape to prove it.
//
// The ids are PocketBase's deterministic ones (type + crc32 of the name), the
// same ids PocketBase assigns to the fields pb-nats adds. A different id would be
// the trap CLAUDE.md describes: the re-import would keep the live field and
// silently ignore this definition.
//
// Values set before this cannot be recovered; they were never stored. A role
// that needs to answer requests must be re-saved with the toggle on.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping role response fields import")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ nats_roles response permission fields declared")
		return nil
	}, nil)
}
