package migrations

import (
	"fmt"
	"log"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_subject_token_to_code re-saves every managed organization so the
// managed-export hook rewrites its hub import's local_subject from
// helpdesk.{org.Id}.> to helpdesk.{org.code}.> (ADR 0002, step 3).
//
// The hook reconciles only when it fires, so each managed org is touched once.
// It re-saves the org rather than updating nats_account_imports directly:
// pb-nats republishes the account JWT from its record hooks, and a raw UPDATE
// would leave NATS enforcing the old import.
//
// Must run after schema_update_org_code.go, which it does by filename. The hook
// refuses an org with no code rather than writing "helpdesk..>".
//
// Deploy order across repos: consumers must resolve codes BEFORE this runs. A
// consumer acks an unresolved org without retrying, so events in the gap are
// dropped. Readers before writers.
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
			// Fires syncManaged → ensureManagedExports. Its failures are logged,
			// not returned, so one unprovisioned org cannot block the rest.
			if err := app.Save(org); err != nil {
				return fmt.Errorf("re-save organization %q: %w", org.GetString("name"), err)
			}
		}

		log.Printf("✅ re-saved %d managed organization(s) to move their import subject onto the org code", len(orgs))
		return nil
	}, func(app core.App) error {
		// No-op: the hook builds the subject, so reverting is a code change.
		return nil
	})
}
