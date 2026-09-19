package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_audit_changed_fields re-imports the embedded schema.json so
// existing deployments pick up `changed_fields` on `audit_logs`.
//
// WHAT THE COLUMN IS FOR. pb-audit used to write a full before/after snapshot
// of every record into every audit row, and a snapshot is PublicExport() --
// every field the collection does not mark `hidden`. Two of this platform's
// fields are deliberately NOT hidden, because the identity that owns them has
// to read them back: `nats_users.creds_file`, which contains the user's NKEY
// seed, and `nebula_hosts.config_yaml`, which carries the host private key
// inline. Row scoping is what protects those on their own collections
// (see the credential note in CLAUDE.md) -- and `audit_logs` has no row
// scoping, so every credential this platform ever minted was sitting in it in
// plaintext, exempt from at-rest encryption (which covers the `seed` and
// `private_key` columns, the hidden ones) and never expiring, since retention
// is off by default.
//
// `changed_fields` names the fields that moved instead. main.go now lists the
// collections whose VALUES are still worth storing, and the credential-bearing
// ones are not among them.
//
// NO BACKFILL, AND NOTHING IS REMOVED. Two deliberate omissions:
//
//   - Nothing can backfill `changed_fields` for rows written before it existed.
//     The information is derivable from their before/after snapshots, but those
//     rows are exactly the ones being kept for their values, so computing it
//     would add a column beside data that already says the same thing.
//
//   - This does NOT delete the historical snapshots. That is a data decision
//     with an operational cost -- it is the audit trail -- and it belongs to
//     whoever runs the deployment, not to a migration that runs silently on
//     deploy. It is worth doing: until those rows are cleared or aged out, the
//     credential archive described above is still in the database. Either set
//     audit.retention in config.yaml and let it age out, or clear the two
//     columns for the credential-bearing collections by hand:
//
//     UPDATE audit_logs SET before_changes = NULL, after_changes = NULL
//     WHERE collection_name IN ('nats_users','nebula_hosts','nats_accounts',
//     'nebula_ca','invites');
//
//     Do it deliberately, with a backup, once you have decided the trail can
//     lose those values.
//
// The field id is PocketBase's deterministic one (`json` + crc32 of the name),
// which is what pb-audit's own ensureAuditCollection produces when it creates
// the collection on a virgin database. They have to agree: core.ImportCollections
// re-adds any existing field the import does not carry BY ID, and FieldsList.add
// then keeps the live definition -- so a hand-picked id here would make this
// migration log its tick and change nothing. See the re-import note in CLAUDE.md.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping audit changed_fields import")
			return nil
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ audit_logs.changed_fields added; record values are now opt-in per collection")
		return nil
	}, nil)
}
