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

// field is one desired value on a managed export/import record. A slice rather
// than a map so a reconcile logs its changed fields in a stable order.
type field struct{ name, value string }

// reconcile applies desired values to rec and returns the names of the fields it
// changed. It saves nothing: an empty result lets the caller skip the write, so
// a no-op org update does not churn the account JWT that pb-nats republishes
// whenever an export or import record is saved.
func reconcile(rec *core.Record, desired []field) []string {
	var changed []string
	for _, f := range desired {
		if rec.GetString(f.name) != f.value {
			rec.Set(f.name, f.value)
			changed = append(changed, f.name)
		}
	}
	return changed
}

// ensureManagedExports brings the export/import pair for a managed org to its
// desired state: it creates the pair when absent and reconciles it when it has
// drifted. Errors are returned for logging, never fatal — an operator can retry
// by toggling the flag once the missing piece (account, hub, key) exists.
//
// It used to only create, and never update what it had created. That was
// survivable while the import's local_subject embedded org.Id, which cannot
// change — but every other value in the pair can. The export subject comes from
// config, the imported account is a public key that can rotate, and both
// descriptions embed a renameable org name.
//
// It stops being survivable once the subject roots at organizations.code
// (ADR 0002, step 3): a stale local_subject would then be an outage that reports
// nothing at all, because a consumer's filter is helpdesk.*.tickets.> and the
// wildcard keeps matching a subject that no longer carries any traffic.
//
// Note for that step: reconciliation only runs when this hook fires, so changing
// the token shape does not retroactively fix existing imports. Whatever makes
// that switch has to touch every managed org, or re-save them.
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

	// account_id and name are the lookup keys below, so they are create-only by
	// definition. Everything else is desired state — including `organization`,
	// whose reconcile doubles as the backfill for records made before org
	// stamping, which the tenant-scoped list rule needs or they stay invisible.
	exportFields := []field{
		{"organization", org.Id},
		{"subject", opts.ExportSubject},
		{"type", "stream"},
		{"description", "Service events exported to the operator hub account"},
	}

	existingExport, _ := app.FindFirstRecordByFilter(exportCol.Id, "account_id = {:acct} && name = {:name}", map[string]interface{}{
		"acct": account.Id,
		"name": managedExportName,
	})
	if existingExport == nil {
		rec := core.NewRecord(exportCol)
		rec.Set("account_id", account.Id)
		rec.Set("name", managedExportName)
		// Create-only: a policy flag on the export rather than a value derived
		// from org state or config, so it cannot drift and is not reconciled.
		rec.Set("token_req", false)
		reconcile(rec, exportFields)
		if err := app.Save(rec); err != nil {
			return fmt.Errorf("create export: %w", err)
		}
		log.Printf("✅ Exported '%s' from org '%s'", opts.ExportSubject, org.GetString("name"))
	} else if changed := reconcile(existingExport, exportFields); len(changed) > 0 {
		if err := app.Save(existingExport); err != nil {
			return fmt.Errorf("reconcile export: %w", err)
		}
		log.Printf("✅ Reconciled export for org '%s' (%s)", org.GetString("name"), strings.Join(changed, ", "))
	}

	// The org token on the subject is the organization CODE, not its id (ADR
	// 0002). The code is globally unique and frozen, so it identifies the tenant
	// as unforgeably as the id did — provenance comes from this rewrite being
	// operator-signed, never from what the token contains — while also being
	// legible in a monitored subject and the same handle every other consumer
	// already resolves by.
	//
	// The record stays NAMED by org.Id. Keeping record identity on the immutable
	// id and the mutable-until-frozen value on the subject is what makes this
	// change a reconcile rather than a delete-and-recreate, and it is why the
	// lookup below still finds imports created under the old token.
	orgCode := org.GetString("code")
	if orgCode == "" {
		return fmt.Errorf("organization has no code yet (run `migrate up`)")
	}

	importName := hubImportName(org.Id)
	localSubject := strings.TrimSuffix(opts.ExportSubject, ".>") + "." + orgCode + ".>"
	importFields := []field{
		{"organization", operatorOrg.Id},
		{"subject", opts.ExportSubject},
		{"account", publicKey},
		{"local_subject", localSubject},
		{"type", "stream"},
		{"description", fmt.Sprintf("Service events from managed org '%s'", org.GetString("name"))},
	}

	existingImport, _ := app.FindFirstRecordByFilter(importCol.Id, "account_id = {:acct} && name = {:name}", map[string]interface{}{
		"acct": hubAccount.Id,
		"name": importName,
	})
	if existingImport == nil {
		rec := core.NewRecord(importCol)
		rec.Set("account_id", hubAccount.Id)
		rec.Set("name", importName)
		reconcile(rec, importFields)
		if err := app.Save(rec); err != nil {
			return fmt.Errorf("create hub import: %w", err)
		}
		log.Printf("✅ Imported org '%s' events into hub as '%s'", org.GetString("name"), localSubject)
	} else if changed := reconcile(existingImport, importFields); len(changed) > 0 {
		if err := app.Save(existingImport); err != nil {
			return fmt.Errorf("reconcile hub import: %w", err)
		}
		log.Printf("✅ Reconciled hub import for org '%s' (%s)", org.GetString("name"), strings.Join(changed, ", "))
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
