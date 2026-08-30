package migrations

import (
	"fmt"
	"log"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_subject_token_to_code re-saves every managed organization so the
// managed-export hook rewrites its hub import's local_subject from
// helpdesk.{org.Id}.> to helpdesk.{org.code}.> (ADR 0002, step 3).
//
// The hook reconciles, but only when it fires. Changing the token shape in code
// does nothing to imports that already exist, so something has to touch each
// managed org once. That is all this is.
//
// It re-saves the ORG rather than updating nats_account_imports directly, and
// the difference is not stylistic. pb-nats regenerates and republishes the
// affected account JWT from its own record hooks on the export/import
// collections. A raw SQL UPDATE would change this database and leave the running
// NATS server still enforcing the old import — the subject would look correct in
// the admin UI and carry nothing on the wire, which is the exact failure mode
// ADR 0002 is trying to remove rather than introduce.
//
// Ordering: this must run after schema_update_org_code.go, which creates and
// backfills the codes. It does, by filename — "org_code" sorts before
// "subject_token_to_code". The hook itself refuses an org with no code rather
// than writing "helpdesk..>", so a mis-ordering would be loud rather than
// silently wrong.
//
// Deploy order across repos is the other half, and it does not live in any
// migration: a consumer resolves subject token 2 against its own tenant table,
// and an unresolved org is ACKED, not retried. Consumers must be able to resolve
// codes BEFORE this runs, or every machine-filed event in the gap is dropped
// with only a log line. Readers before writers.
func init() {
	m.Register(func(app core.App) error {
		orgs, err := app.FindAllRecords("organizations", dbx.HashExp{"managed": true})
		if err != nil {
			return fmt.Errorf("find managed organizations: %w", err)
		}
		if len(orgs) == 0 {
			log.Println("✅ no managed organizations to re-subject")
			return nil
		}

		for _, org := range orgs {
			// Fires OnRecordAfterUpdateSuccess → syncManaged → ensureManagedExports,
			// whose reconcile logs exactly which fields it changed. Failures there
			// are logged and not propagated, so one unprovisioned org cannot block
			// the migration for the rest.
			if err := app.Save(org); err != nil {
				return fmt.Errorf("re-save organization %q: %w", org.GetString("name"), err)
			}
		}

		log.Printf("✅ re-saved %d managed organization(s) to move their import subject onto the org code", len(orgs))
		return nil
	}, func(app core.App) error {
		// Down: no-op. Reverting means putting the id back on the subject, which
		// is a code change (the hook builds it), not a data change.
		return nil
	})
}
