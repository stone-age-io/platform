package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_privesc_chain closes a privilege-escalation chain that let any
// authenticated user reach platform-operator and from there read the NATS root
// operator seed. Re-imports the embedded schema.json, then remediates data that
// the schema change alone cannot fix.
//
// SCHEMA CHANGES (in schema.json):
//
//   - users.updateRule: added `@request.body.is_operator:isset = false`. The rule
//     previously guarded only current_organization, so any authenticated user
//     could PATCH is_operator=true on their own record and become a platform
//     operator. Nothing else guarded it — there are no hooks on `users`.
//   - users.createRule: the anonymous branch now requires a pending invite for
//     the submitted address. It was open, unauthenticated account creation.
//   - memberships.updateRule: the self-update branch was a bare
//     `user = @request.auth.id`, which allowed PATCH {"role":"owner"} on your own
//     membership. It is now restricted to the nats_user link, and no branch may
//     re-point a membership at another organization.
//   - 30 rules across 12 collections: `role ?!= "member"` -> an explicit
//     `role ?= "owner" || role ?= "admin"` allowlist. The deny-list form was also
//     satisfied by role="badge" — the most restricted role — so badge holders
//     passed every org-admin check. An allowlist also fails safe for roles added
//     later.
//   - nats_system_operator.{private_key,seed,signing_private_key,signing_seed}:
//     hidden=true, matching how nats_accounts and nebula_ca already treat their
//     equivalents. Server-side Go reads are unaffected (hidden applies to the
//     HTTP layer); PublicExport() now omits them, which is what stops them being
//     copied into audit_logs.
//
// DATA REMEDIATION (below): existing audit_logs rows already hold plaintext
// copies of the operator seed, written before the hidden flags existed. The
// hidden flag only affects future writes, so those payloads are redacted here.
//
// NOTE: redaction is not key rotation. On any deployment where an untrusted
// party could have read audit_logs, the operator seed must be considered
// disclosed and the NATS operator key rotated — that cannot be done in a
// migration because every account JWT has to be re-signed and redistributed.
//
// Additive import (deleteMissing=false); safe on fresh DBs.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping privilege-escalation update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Privilege-escalation chain closed: is_operator frozen, membership self-escalation blocked, role checks switched to an allowlist, operator secrets hidden")

		redactLeakedOperatorSecrets(app)
		reportOperatorAccounts(app)

		return nil
	}, nil)
}

// redactLeakedOperatorSecrets strips the before/after payloads from audit rows
// that captured the nats_system_operator record. Those payloads contain the
// operator seed and signing seed in plaintext.
//
// It blanks only the two change payloads and leaves the surrounding row intact,
// so the audit trail still records that the operator record was touched, by
// whom, and when — the compliance value is preserved, the secret is not.
//
// Assumes the default audit collection name. A deployment that overrode
// audit.collection_name should redact its own table by hand.
func redactLeakedOperatorSecrets(app core.App) {
	const auditCollection = "audit_logs"

	if _, err := app.FindCollectionByNameOrId(auditCollection); err != nil {
		return // no audit collection on this deployment; nothing to redact
	}

	const redacted = `"[redacted: contained the NATS operator seed in plaintext]"`

	// Raw SQL on purpose: this must not fan out into record hooks (which would
	// re-audit the rows we are scrubbing).
	res, err := app.DB().NewQuery(`
		UPDATE ` + auditCollection + `
		SET before_changes = {:redacted}, after_changes = {:redacted}
		WHERE collection_name = 'nats_system_operator'
		  AND (before_changes IS NOT NULL OR after_changes IS NOT NULL)
	`).Bind(map[string]any{"redacted": redacted}).Execute()
	if err != nil {
		// Non-fatal: the schema fix is the important half, and failing the
		// migration would leave the deployment on the vulnerable rules.
		log.Printf("⚠️ Could not redact leaked operator secrets from %s: %v", auditCollection, err)
		log.Printf("⚠️ Redact by hand: UPDATE %s SET before_changes=NULL, after_changes=NULL WHERE collection_name='nats_system_operator';", auditCollection)
		return
	}

	if n, err := res.RowsAffected(); err == nil && n > 0 {
		log.Printf("🧹 Redacted plaintext NATS operator secrets from %d %s row(s)", n, auditCollection)
		log.Printf("⚠️ The operator seed was readable by any operator account before this upgrade. If an untrusted party may have held one, rotate the NATS operator key — redaction is not rotation.")
	}
}

// reportOperatorAccounts lists platform operators so an upgrading admin can spot
// accounts that self-granted the flag while users.updateRule was open. It only
// reports: revoking automatically would risk locking out the legitimate operator
// created by `bootstrap`.
func reportOperatorAccounts(app core.App) {
	records, err := app.FindRecordsByFilter("users", "is_operator = true", "created", 0, 0)
	if err != nil {
		log.Printf("⚠️ Could not enumerate operator accounts for review: %v", err)
		return
	}

	log.Printf("🔎 %d platform operator account(s) exist — verify each one is expected:", len(records))
	for _, r := range records {
		log.Printf("     • %s (id=%s, created=%s)", r.Email(), r.Id, r.GetDateTime("created").String())
	}
	if len(records) > 1 {
		log.Println("⚠️ More than one operator: until this upgrade any authenticated user could grant themselves this flag. Confirm every account above was created deliberately.")
	}
}
