package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_authz_hardening re-imports the embedded schema.json so existing
// deployments pick up tightened authorization rules:
//
//   - things / nats_users / nebula_hosts deleteRule: was "@request.auth.id != ''",
//     which let ANY authenticated identity (any org's user, or a thing/identity
//     auth record) delete ANY record cross-tenant. Now scoped to the caller's
//     active organization, mirroring each collection's updateRule.
//   - audit_logs list/view: was "@request.auth.id != ''", exposing every tenant's
//     audit trail (including before/after change payloads) to any authenticated
//     identity. The collection has no organization field, so reads are now
//     restricted to operators.
//
// Additive import (deleteMissing=false); safe on fresh DBs.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping authz hardening update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Authz hardening: scoped delete rules + operator-only audit reads applied")
		return nil
	}, nil)
}
