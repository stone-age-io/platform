package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_role_response_fields declares nats_roles.allow_response,
// allow_response_max and allow_response_ttl, which schema.json never listed.
//
// A database created before pb-nats added these fields (January 2026) never got
// them, and PocketBase silently drops writes to an undeclared field, so the
// console's "Response Permissions" toggle stored nothing there.
//
// pb-nats v0.3.0 adds missing role fields at startup; this declares them anyway
// so `migrate up` alone yields the right schema. role_response_fields_test.go
// builds the legacy shape to prove it.
//
// The ids are PocketBase's deterministic type + crc32 of the name; any other id
// would be silently ignored by the re-import (migrations/CLAUDE.md).
//
// Values set before this were never stored. A role that must answer requests
// has to be re-saved with the toggle on.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			return errNoSchema
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ nats_roles response permission fields declared")
		return nil
	}, nil)
}
