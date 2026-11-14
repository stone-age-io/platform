package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_account_key_routes makes nats_accounts.updateRule and
// nebula_ca.updateRule `@request.auth.is_operator = true`, and moves the
// signing-key operations owners and admins need into a route.
//
// Both rules had an owner/admin branch made of `:changed = false` assertions on
// the fields meant to stay closed: a deny-list, so every unnamed field was open,
// including any added later. On nats_accounts that left `jwt`, `revocations` and
// the signing keys writable; on nebula_ca it left `certificate` and `expires_at`,
// so a tenant could replace its own overlay trust anchor. A rule cannot express
// "this one field and nothing else".
//
// POST /api/org/nats-account/keys (hooks/nats_account_routes.go) takes no record
// id, derives the account from the caller's organization, and maps each action
// (rotate, add_signing, remove_signing) to exactly one field. Account limits stay
// operator-only: they are the envelope the tenant was sold. CA rotation is on a
// route for the same reason (POST /api/org/nebula-ca/rotate, hooks/nebula_routes.go).
// Reads are unchanged.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			return errNoSchema
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ nats_accounts + nebula_ca updates are operator-only; tenant key operations moved to POST /api/org/nats-account/keys")

		reportTenantWritableAccountFields(app)

		return nil
	}, nil)
}

// reportTenantWritableAccountFields tells an upgrading operator how many accounts
// and CAs existed under the deny-list branches. It cannot tell whether any was
// changed, so it points at audit_logs.
func reportTenantWritableAccountFields(app core.App) {
	for _, c := range []struct{ collection, note string }{
		{"nats_accounts", "jwt / revocations / signing keys were writable by any owner or admin"},
		{"nebula_ca", "certificate / expires_at were writable by any owner or admin"},
	} {
		if _, err := app.FindCollectionByNameOrId(c.collection); err != nil {
			continue
		}
		records, err := app.FindRecordsByFilter(c.collection, "1=1", "created", 0, 0)
		if err != nil {
			log.Printf("⚠️ Could not review %s: %v", c.collection, err)
			continue
		}
		if len(records) == 0 {
			continue
		}
		log.Printf("🔎 %d %s record(s) predate this upgrade — until now, %s. Check audit_logs for updates to these records if you need assurance none were tampered with.",
			len(records), c.collection, c.note)
	}
}
