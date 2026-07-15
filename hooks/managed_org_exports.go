package hooks

import (
	"fmt"
	"log"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// ManagedOrgExportsOptions captures the collections and subject space used to
// wire a managed organization's NATS account to the operator hub account.
type ManagedOrgExportsOptions struct {
	OrgCollection     string
	AccountCollection string
	ExportCollection  string
	ImportCollection  string

	// ExportSubject is the app-token subject subtree exported from every
	// managed org's account (must end in ".>", e.g. "helpdesk.>").
	ExportSubject string
}

const managedExportName = "helpdesk-events"

// RegisterManagedOrgExports attaches hooks that keep a managed organization's
// cross-account wiring in sync with its `managed` flag:
//
//   - managed = true  → a stream export of ExportSubject on the org's account,
//     plus an import of it on the operator hub account remapped to an
//     org-prefixed local subject ("helpdesk.>" → "helpdesk.{orgId}.>").
//     The prefix lives in the hub's signed account JWT, so the org identity
//     on the subject cannot be forged by the publisher.
//   - managed = false (or org deleted) → the pair is removed.
//
// pb-nats owns the rest: its export/import record hooks regenerate and
// republish the affected account JWTs.
func RegisterManagedOrgExports(app *pocketbase.PocketBase, opts ManagedOrgExportsOptions) {
	syncManaged := func(e *core.RecordEvent) error {
		org := e.Record
		// The system org never carries app traffic; the operator org IS the hub.
		if org.GetBool("is_system_org") || org.GetBool("is_operator_org") || org.GetString("name") == "System" {
			return e.Next()
		}
		if org.GetBool("managed") {
			if err := ensureManagedExports(e.App, opts, org); err != nil {
				log.Printf("❌ Failed to provision managed exports for '%s': %v", org.GetString("name"), err)
			}
		} else {
			removeManagedExports(e.App, opts, org)
		}
		return e.Next()
	}

	app.OnRecordAfterCreateSuccess(opts.OrgCollection).BindFunc(syncManaged)
	app.OnRecordAfterUpdateSuccess(opts.OrgCollection).BindFunc(syncManaged)

	// The org's own export cascade-deletes with its account, but the hub-side
	// import would otherwise be orphaned.
	app.OnRecordAfterDeleteSuccess(opts.OrgCollection).BindFunc(func(e *core.RecordEvent) error {
		removeManagedExports(e.App, opts, e.Record)
		return e.Next()
	})
}

// ensureManagedExports idempotently creates the export/import pair for a
// managed org. Errors are returned for logging, never fatal — an operator can
// retry by toggling the flag once the missing piece (account, hub, key) exists.
func ensureManagedExports(app core.App, opts ManagedOrgExportsOptions, org *core.Record) error {
	account, _ := app.FindFirstRecordByFilter(opts.AccountCollection, "organization = {:org}", map[string]interface{}{"org": org.Id})
	if account == nil {
		return fmt.Errorf("no NATS account for organization yet")
	}
	publicKey := account.GetString("public_key")
	if publicKey == "" {
		return fmt.Errorf("NATS account has no public key yet")
	}

	operatorOrg, _ := app.FindFirstRecordByFilter(opts.OrgCollection, "is_operator_org = true")
	if operatorOrg == nil {
		return fmt.Errorf("no operator organization exists (re-run bootstrap with --operator-org)")
	}
	hubAccount, _ := app.FindFirstRecordByFilter(opts.AccountCollection, "organization = {:org}", map[string]interface{}{"org": operatorOrg.Id})
	if hubAccount == nil {
		return fmt.Errorf("operator organization has no NATS account")
	}

	exportCol, err := app.FindCollectionByNameOrId(opts.ExportCollection)
	if err != nil {
		return fmt.Errorf("find export collection: %w", err)
	}
	importCol, err := app.FindCollectionByNameOrId(opts.ImportCollection)
	if err != nil {
		return fmt.Errorf("find import collection: %w", err)
	}

	existingExport, _ := app.FindFirstRecordByFilter(exportCol.Id, "account_id = {:acct} && name = {:name}", map[string]interface{}{
		"acct": account.Id,
		"name": managedExportName,
	})
	if existingExport == nil {
		rec := core.NewRecord(exportCol)
		rec.Set("account_id", account.Id)
		rec.Set("organization", org.Id)
		rec.Set("name", managedExportName)
		rec.Set("subject", opts.ExportSubject)
		rec.Set("type", "stream")
		rec.Set("token_req", false)
		rec.Set("description", "Service events exported to the operator hub account")
		if err := app.Save(rec); err != nil {
			return fmt.Errorf("create export: %w", err)
		}
		log.Printf("✅ Exported '%s' from org '%s'", opts.ExportSubject, org.GetString("name"))
	} else if existingExport.GetString("organization") != org.Id {
		// Backfill records created before organization stamping — otherwise the
		// tenant-scoped list rule keeps them invisible in the UI.
		existingExport.Set("organization", org.Id)
		if err := app.Save(existingExport); err != nil {
			return fmt.Errorf("backfill export organization: %w", err)
		}
		log.Printf("✅ Backfilled organization on export for org '%s'", org.GetString("name"))
	}

	importName := hubImportName(org.Id)
	existingImport, _ := app.FindFirstRecordByFilter(importCol.Id, "account_id = {:acct} && name = {:name}", map[string]interface{}{
		"acct": hubAccount.Id,
		"name": importName,
	})
	if existingImport == nil {
		localSubject := strings.TrimSuffix(opts.ExportSubject, ".>") + "." + org.Id + ".>"
		rec := core.NewRecord(importCol)
		rec.Set("account_id", hubAccount.Id)
		rec.Set("organization", operatorOrg.Id)
		rec.Set("name", importName)
		rec.Set("subject", opts.ExportSubject)
		rec.Set("account", publicKey)
		rec.Set("local_subject", localSubject)
		rec.Set("type", "stream")
		rec.Set("description", fmt.Sprintf("Service events from managed org '%s'", org.GetString("name")))
		if err := app.Save(rec); err != nil {
			return fmt.Errorf("create hub import: %w", err)
		}
		log.Printf("✅ Imported org '%s' events into hub as '%s'", org.GetString("name"), localSubject)
	} else if existingImport.GetString("organization") != operatorOrg.Id {
		// Backfill records created before organization stamping — the hub import
		// belongs to the operator org and is otherwise invisible there.
		existingImport.Set("organization", operatorOrg.Id)
		if err := app.Save(existingImport); err != nil {
			return fmt.Errorf("backfill hub import organization: %w", err)
		}
		log.Printf("✅ Backfilled organization on hub import for org '%s'", org.GetString("name"))
	}

	return nil
}

// removeManagedExports tears down the export/import pair for an org that is
// no longer managed (or was deleted). Missing pieces are silently fine.
func removeManagedExports(app core.App, opts ManagedOrgExportsOptions, org *core.Record) {
	if account, _ := app.FindFirstRecordByFilter(opts.AccountCollection, "organization = {:org}", map[string]interface{}{"org": org.Id}); account != nil {
		rec, _ := app.FindFirstRecordByFilter(opts.ExportCollection, "account_id = {:acct} && name = {:name}", map[string]interface{}{
			"acct": account.Id,
			"name": managedExportName,
		})
		if rec != nil {
			if err := app.Delete(rec); err != nil {
				log.Printf("❌ Failed to delete export for '%s': %v", org.GetString("name"), err)
			} else {
				log.Printf("🗑️ Removed '%s' export from org '%s'", opts.ExportSubject, org.GetString("name"))
			}
		}
	}

	// The import name embeds the org id, so it's findable without the hub account.
	rec, _ := app.FindFirstRecordByFilter(opts.ImportCollection, "name = {:name}", map[string]interface{}{"name": hubImportName(org.Id)})
	if rec != nil {
		if err := app.Delete(rec); err != nil {
			log.Printf("❌ Failed to delete hub import for '%s': %v", org.GetString("name"), err)
		} else {
			log.Printf("🗑️ Removed hub import for org '%s'", org.GetString("name"))
		}
	}
}

func hubImportName(orgID string) string {
	return managedExportName + "-" + orgID
}
