package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_account_key_routes makes nats_accounts.updateRule and
// nebula_ca.updateRule platform-operator-only, and moves the signing-key
// operations an organization's owners and admins need into a dedicated route.
//
// THE PROBLEM. Both rules carried an owner/admin branch commented "can only
// change rotate_keys", implemented as a list of `:changed = false` assertions on
// the fields that were meant to stay closed. That is a deny-list, and it is the
// third instance of the same failure in this codebase — after `role ?!= "member"`
// (satisfied by `badge`) and the `things`/`locations` write branches that
// restricted fields without naming a role. A rule cannot express "this one field
// and nothing else"; naming the fields to freeze means everything unnamed is open,
// including every field added afterwards.
//
// What the two branches actually permitted:
//
//   - nats_accounts: `jwt` (the signed account JWT), `revocations` (which user
//     JWTs the account rejects), `public_key`, `signing_public_key`,
//     `signing_keys`, `name`, `description`, `active`. Only the six `max_*` limit
//     fields and `organization` were frozen.
//   - nebula_ca: `name`, `certificate`, `expires_at` — an owner or admin could
//     replace the organization's Nebula CA certificate, the trust anchor for its
//     whole overlay network. Only `validity_years`, `curve` and `organization`
//     were frozen. Note the branch was commented "can only change rotate_keys"
//     while nebula_ca HAS NO rotate_keys FIELD, so that comment described an
//     operation the rule could not perform and the collection does not support.
//
// This was not a cross-role escalation — owner and admin already administer NATS
// and Nebula — but both rules permitted far more than they claimed, which makes
// them impossible to review, and the silent-widening-on-new-field property is the
// part that actually bites.
//
// THE REPLACEMENT. Both update rules are now `@request.auth.is_operator = true`.
// The three operations a tenant legitimately performs on its own account moved to
//
//	POST /api/org/nats-account/keys   { "action": "rotate" | "add_signing" | "remove_signing" }
//
// (hooks/nats_account_routes.go), gated to owner/admin of the caller's active
// organization. It takes no record id — the account is derived from the caller's
// own organization — and a switch statement maps each action to exactly one field,
// rejecting anything else. That switch is the allowlist a rule could not express.
//
// The account limits stay operator-only deliberately: they are the resource
// envelope the tenant was sold, so raising them is not a tenant action. Nebula CA
// rotation has no route because it has no trigger field; rolling a CA is an
// operator operation.
//
// NOT AFFECTED: reads. Both collections remain readable by any role in the
// organization, which is what the console's account and CA detail views rely on.
// The operator-gated limits editor in admin/OrganizationDetailView.vue keeps
// working because operators still satisfy the rule.
//
// Additive import (deleteMissing=false); safe on fresh DBs.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping account-key-route update")
			return nil
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
// and CAs existed while the deny-list branches were in force, so they know the
// scope of what to review. It cannot tell whether anything was actually changed —
// that is what audit_logs is for — so it only points at where to look.
func reportTenantWritableAccountFields(app core.App) {
	for _, c := range []struct{ collection, note string }{
		{"nats_accounts", "jwt / revocations / signing keys were writable by any owner or admin"},
		{"nebula_ca", "certificate / expires_at were writable by any owner or admin"},
	} {
		if _, err := app.FindCollectionByNameOrId(c.collection); err != nil {
			continue // collection not present on this deployment
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
