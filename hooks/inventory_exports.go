package hooks

import (
	"fmt"
	"log"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// The export/import pair that carries inventory events from the provider
// (operator) account into each organization's own account -- the mirror image
// of managed_org_exports.go, with the two accounts swapped.
//
//   - Export, on the provider account, named inventory-<org.Id>, subject
//     <prefix>.<org-code>.>
//   - Import, on the org's account, named inventory-events, same subject,
//     remapped to LocalSubject (stone-age.>)
//
// Every organization gets one while nats.inventory_events_enabled is on, not
// only managed ones: the helpdesk export lets the provider reach INTO a tenant,
// and this one hands a tenant its own data. One export per org rather than one
// wildcard export, because a wildcard cannot be revoked for one tenant.
//
// Records are NAMED by org.Id and their subjects keyed on org.code, for the
// reason managed_org_exports.go gives: identity on the immutable id, the
// addressable value on the subject, so a change is a reconcile.
//
// The console shows both records read-only, recognising them by these names
// (ui/src/utils/managedExports.ts). Keep the two in step.

const (
	inventoryExportPrefix = "inventory-"
	inventoryImportName   = "inventory-events"
)

func inventoryExportName(orgID string) string {
	return inventoryExportPrefix + orgID
}

// isInfrastructureOrg is the system org, which never carries app traffic, or
// the operator org, which IS the provider account and needs no import from
// itself.
func isInfrastructureOrg(org *core.Record) bool {
	return org.GetBool("is_system_org") || org.GetBool("is_operator_org") || org.GetString("name") == "System"
}

func registerInventoryExports(app *pocketbase.PocketBase, opts InventoryEventsOptions) {
	if opts.Enabled {
		// Bound to update as well as create, as the managed pair is: re-saving
		// an organization is how a failed provision is retried, and a no-op
		// reconcile writes nothing.
		syncOrg := func(e *core.RecordEvent) error {
			if !isInfrastructureOrg(e.Record) {
				if err := ensureInventoryExports(e.App, opts, e.Record); err != nil {
					log.Printf("❌ Failed to wire inventory events for '%s': %v", e.Record.GetString("name"), err)
				}
			}
			return e.Next()
		}
		app.OnRecordAfterCreateSuccess(opts.OrgCollection).BindFunc(syncOrg)
		app.OnRecordAfterUpdateSuccess(opts.OrgCollection).BindFunc(syncOrg)
	}

	// BEFORE the delete, not after it, and inside its transaction (e.App), so a
	// refused delete rolls the removal back with it. Deleting an organization
	// blanks rather than cascades: PocketBase unsets `organization` on each
	// referencing record one at a time, and for a moment the import's
	// organization and its account's disagree -- which relation tenancy refuses,
	// failing the whole delete. Removing the pair first leaves nothing to
	// disagree. Bound whether or not the flag is on, so a pair left by an
	// earlier enabled run can never block a delete.
	app.OnRecordDelete(opts.OrgCollection).BindFunc(func(e *core.RecordEvent) error {
		removeInventoryExports(e.App, opts, e.Record)
		return e.Next()
	})

	// The flag is config, so turning it on or off takes effect on restart. This
	// is also how existing organizations are wired on the first boot after an
	// upgrade.
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		sweepInventoryExports(e.App, opts)
		return e.Next()
	})
}

// sweepInventoryExports ensures the pair on every organization when the flag is
// on, and removes every pair when it is off. Reconcile writes nothing when a
// record is already right, which on a sweep across every org is the difference
// between a quiet restart and republishing every account JWT.
func sweepInventoryExports(app core.App, opts InventoryEventsOptions) {
	orgs, err := app.FindAllRecords(opts.OrgCollection)
	if err != nil {
		log.Printf("❌ Inventory export sweep could not list organizations: %v", err)
		return
	}
	for _, org := range orgs {
		if isInfrastructureOrg(org) {
			continue
		}
		if !opts.Enabled {
			removeInventoryExports(app, opts, org)
			continue
		}
		if err := ensureInventoryExports(app, opts, org); err != nil {
			log.Printf("❌ Failed to wire inventory events for '%s': %v", org.GetString("name"), err)
		}
	}
}

// ensureInventoryExports creates the pair for one org when absent and
// reconciles it when it has drifted. Errors are returned for logging, never
// fatal.
func ensureInventoryExports(app core.App, opts InventoryEventsOptions, org *core.Record) error {
	orgCode := org.GetString("code")
	if orgCode == "" {
		return fmt.Errorf("organization has no code yet (run `migrate up`)")
	}
	account, _ := app.FindFirstRecordByFilter(opts.AccountCollection, "organization = {:org}", dbx.Params{"org": org.Id})
	if account == nil {
		return fmt.Errorf("no NATS account for organization yet")
	}
	operatorOrg, hubAccount, err := operatorHub(app, opts.OrgCollection, opts.AccountCollection)
	if err != nil {
		return err
	}
	hubKey := hubAccount.GetString("public_key")
	if hubKey == "" {
		return fmt.Errorf("operator NATS account has no public key yet")
	}

	subject := inventorySubjectRoot(opts.LocalSubject, orgCode) + ".>"

	// The export sits on the provider account, so it carries the OPERATOR org:
	// relation tenancy refuses a record whose organization differs from its
	// account's.
	if err := ensureNamedRecord(app, opts.ExportCollection, hubAccount.Id, inventoryExportName(org.Id),
		map[string]any{"token_req": false},
		[]field{
			{"organization", operatorOrg.Id},
			{"subject", subject},
			{"type", "stream"},
			{"description", fmt.Sprintf("Inventory events for organization '%s'", org.GetString("name"))},
		}); err != nil {
		return fmt.Errorf("inventory export: %w", err)
	}

	if err := ensureNamedRecord(app, opts.ImportCollection, account.Id, inventoryImportName, nil,
		[]field{
			{"organization", org.Id},
			{"subject", subject},
			{"account", hubKey},
			{"local_subject", opts.LocalSubject},
			{"type", "stream"},
			{"description", "Inventory events from the Control Plane"},
		}); err != nil {
		return fmt.Errorf("inventory import: %w", err)
	}
	return nil
}

// ensureNamedRecord finds the record keyed by (account_id, name), creates it
// with createOnly plus desired when absent, and otherwise saves only if
// reconcile changed something -- every save makes pb-nats republish the
// account JWT.
func ensureNamedRecord(app core.App, collection, accountID, name string, createOnly map[string]any, desired []field) error {
	existing, _ := app.FindFirstRecordByFilter(collection, "account_id = {:acct} && name = {:name}", dbx.Params{
		"acct": accountID,
		"name": name,
	})
	if existing == nil {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			return err
		}
		rec := core.NewRecord(col)
		rec.Set("account_id", accountID)
		rec.Set("name", name)
		for k, v := range createOnly {
			rec.Set(k, v)
		}
		reconcile(rec, desired)
		if err := app.Save(rec); err != nil {
			return err
		}
		log.Printf("✅ Created %s '%s'", strings.TrimSuffix(collection, "s"), name)
		return nil
	}
	if changed := reconcile(existing, desired); len(changed) > 0 {
		if err := app.Save(existing); err != nil {
			return err
		}
		log.Printf("✅ Reconciled %s '%s' (%s)", strings.TrimSuffix(collection, "s"), name, strings.Join(changed, ", "))
	}
	return nil
}

// removeInventoryExports tears down one org's pair. Missing pieces are fine.
func removeInventoryExports(app core.App, opts InventoryEventsOptions, org *core.Record) {
	// The export name embeds the org id, so it is findable without the hub.
	if rec, _ := app.FindFirstRecordByFilter(opts.ExportCollection, "name = {:name}",
		dbx.Params{"name": inventoryExportName(org.Id)}); rec != nil {
		if err := app.Delete(rec); err != nil {
			log.Printf("❌ Failed to delete inventory export for '%s': %v", org.GetString("name"), err)
		} else {
			log.Printf("🗑️ Removed inventory export for org '%s'", org.GetString("name"))
		}
	}

	account, _ := app.FindFirstRecordByFilter(opts.AccountCollection, "organization = {:org}", dbx.Params{"org": org.Id})
	if account == nil {
		return // deleted with the org, and the import cascaded with it
	}
	if rec, _ := app.FindFirstRecordByFilter(opts.ImportCollection, "account_id = {:acct} && name = {:name}",
		dbx.Params{"acct": account.Id, "name": inventoryImportName}); rec != nil {
		if err := app.Delete(rec); err != nil {
			log.Printf("❌ Failed to delete inventory import for '%s': %v", org.GetString("name"), err)
		} else {
			log.Printf("🗑️ Removed inventory import for org '%s'", org.GetString("name"))
		}
	}
}
