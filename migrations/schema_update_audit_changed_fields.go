package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_audit_changed_fields re-imports the embedded schema.json so
// existing deployments pick up `changed_fields` on `audit_logs`.
//
// Audit rows held a full PublicExport() snapshot of every record, and two
// credential fields are deliberately not hidden: `nats_users.creds_file` (the
// NKEY seed) and `nebula_hosts.config_yaml` (the host key). Row scoping protects
// them on their own collections, but audit_logs has none, so every credential
// ever minted sat there in plaintext, outside at-rest encryption, with no
// retention by default. `changed_fields` names the fields that moved instead,
// and main.go lists the collections whose values are still stored.
//
//   - No backfill: old rows already carry the same information in their
//     snapshots.
//
//   - This does not delete the historical snapshots. That is the audit trail,
//     so it is the operator's decision, not a silent step on deploy. Until those
//     rows are cleared or aged out the credential archive is still there. Set
//     audit.retention in config.yaml, or clear the columns by hand:
//
//     UPDATE audit_logs SET before_changes = NULL, after_changes = NULL
//     WHERE collection_name IN ('nats_users','nebula_hosts','nats_accounts',
//     'nebula_ca','invites');
//
//     Do it deliberately, with a backup, once you have decided the trail can
//     lose those values.
//
// The field id is PocketBase's deterministic one (`json` + crc32 of the name),
// matching what pb-audit's ensureAuditCollection creates; any other id would make
// the re-import keep the live field and change nothing. See the re-import note
// in migrations/CLAUDE.md.
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
