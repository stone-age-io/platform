package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_privesc_chain closes a privilege-escalation chain that let any
// authenticated user become a platform operator and read the NATS root operator
// seed. It re-imports schema.json, then remediates data. Schema changes:
//
//   - users.updateRule: `@request.body.is_operator:isset = false`. Any user could
//     PATCH is_operator=true on their own record; no hook guards `users`.
//   - users.createRule: the anonymous branch requires a pending invite for the
//     address. It was open account creation.
//   - memberships.updateRule: the bare `user = @request.auth.id` self branch
//     allowed PATCH {"role":"owner"}. It is now limited to the nats_user link,
//     and no branch may move a membership to another organization.
//   - 30 rules across 12 collections: `role ?!= "member"` became the allowlist
//     `role ?= "owner" || role ?= "admin"`. The deny-list also passed the most
//     restricted role, and an allowlist fails safe for roles added later.
//   - nats_system_operator.{private_key,seed,signing_private_key,signing_seed}:
//     hidden=true. Go reads are unaffected; PublicExport() now omits them, which
//     keeps them out of audit_logs.
//
// Existing audit_logs rows already hold plaintext copies of the operator seed,
// so they are redacted below. Redaction is not rotation: where an untrusted
// party could have read audit_logs, rotate the NATS operator key by hand, which
// re-signs every account JWT.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			return errNoSchema
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

// redactLeakedOperatorSecrets blanks the before/after payloads of audit rows for
// nats_system_operator, which hold its seeds in plaintext. The rest of the row
// stays, so the trail still says who touched the record and when.
//
// Assumes the default audit collection name. A deployment that overrode
// audit.collection_name should redact its own table by hand.
func redactLeakedOperatorSecrets(app core.App) {
	const auditCollection = "audit_logs"

	if _, err := app.FindCollectionByNameOrId(auditCollection); err != nil {
		return // no audit collection on this deployment; nothing to redact
	}

	const redacted = `"[redacted: contained the NATS operator seed in plaintext]"`

	// Raw SQL so record hooks do not re-audit the rows being scrubbed.
	res, err := app.DB().NewQuery(`
		UPDATE ` + auditCollection + `
		SET before_changes = {:redacted}, after_changes = {:redacted}
		WHERE collection_name = 'nats_system_operator'
		  AND (before_changes IS NOT NULL OR after_changes IS NOT NULL)
	`).Bind(map[string]any{"redacted": redacted}).Execute()
	if err != nil {
		// Non-fatal: failing would roll back the rule fix too.
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
