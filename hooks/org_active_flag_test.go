package hooks_test

import (
	"errors"
	"testing"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"platform/hooks"
	"platform/internal/testutil"
)

// newFlagOrg creates an organization with a fresh owner and hands back the RELOADED
// record. The reload is load-bearing (see TestRelationTenancy): a record saved
// in memory carries an Original() describing the blank pre-create state, so an
// edge-keyed hook never fires, and the suite would pass against a hook that
// refuses nothing. Real callers go through the record API, which loads first.
//
// `slug` builds the owner's address, since names have spaces.
func newFlagOrg(t *testing.T, app *pocketbase.PocketBase, slug, name string, flags map[string]any) *core.Record {
	t.Helper()

	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("users collection: %v", err)
	}
	owner := core.NewRecord(usersCol)
	owner.Set("email", slug+"-owner@example.test")
	owner.Set("password", "owner-password-123")
	if err := app.Save(owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}

	orgCol, err := app.FindCollectionByNameOrId("organizations")
	if err != nil {
		t.Fatalf("organizations collection: %v", err)
	}
	org := core.NewRecord(orgCol)
	org.Set("name", name)
	org.Set("active", true)
	org.Set("owner", owner.Id)
	for k, v := range flags {
		org.Set(k, v)
	}
	if err := app.Save(org); err != nil {
		t.Fatalf("create organization %q: %v", name, err)
	}

	reloaded, err := app.FindRecordById("organizations", org.Id)
	if err != nil {
		t.Fatalf("reload organization %q: %v", name, err)
	}
	return reloaded
}

// newOrgWithAccount is newFlagOrg plus the NATS account provisioning gave it.
func newOrgWithAccount(t *testing.T, app *pocketbase.PocketBase, slug, name string, flags map[string]any) (*core.Record, *core.Record) {
	t.Helper()

	org := newFlagOrg(t, app, slug, name, flags)

	account, err := app.FindFirstRecordByFilter("nats_accounts",
		"organization = {:org}", dbx.Params{"org": org.Id})
	if err != nil {
		t.Fatalf("provisioning should have created a NATS account for %q: %v", name, err)
	}
	return org, account
}

func accountActive(t *testing.T, app *pocketbase.PocketBase, accountID string) bool {
	t.Helper()
	rec, err := app.FindRecordById("nats_accounts", accountID)
	if err != nil {
		t.Fatalf("reload NATS account: %v", err)
	}
	return rec.GetBool("active")
}

// Both directions on the same record: a mirror that only writes false would
// pass a deactivation test while making reactivation, the half that makes this
// safe to expose, impossible.
func TestOrgDeactivationMirrorsOntoTheNatsAccount(t *testing.T) {
	app := testutil.SetupApp(t)

	org, account := newOrgWithAccount(t, app, "mirror", "Mirror Works", nil)

	if !accountActive(t, app, account.Id) {
		t.Fatal("an organization created active should have an active NATS account")
	}

	org.Set("active", false)
	if err := app.Save(org); err != nil {
		t.Fatalf("deactivate the organization: %v", err)
	}
	if accountActive(t, app, account.Id) {
		t.Error("deactivating the organization left its NATS account active: " +
			"the badge says suspended while every device in the tenant is still connected")
	}

	org.Set("active", true)
	if err := app.Save(org); err != nil {
		t.Fatalf("reactivate the organization: %v", err)
	}
	if !accountActive(t, app, account.Id) {
		t.Error("reactivating the organization did not bring its NATS account back")
	}
}

// An organization created with `active` already clear must not be handed a live
// NATS account. The mirror binds to update only, so ensureOrgNatsAccount
// inheriting the flag is what enforces this.
func TestAnOrgCreatedInactiveGetsAnInactiveAccount(t *testing.T) {
	app := testutil.SetupApp(t)

	_, account := newOrgWithAccount(t, app, "born-suspended", "Born Suspended", map[string]any{"active": false})

	if accountActive(t, app, account.Id) {
		t.Error("an organization created with active=false was provisioned a live NATS account")
	}
}

// The mirror is LEVEL-triggered; this fails if it is rewritten as an edge. A
// returned error is only actionable if re-saving retries, and an account
// flipped directly in the PocketBase dashboard must come back into line.
func TestTheMirrorCorrectsDriftRatherThanOnlyWatchingEdges(t *testing.T) {
	app := testutil.SetupApp(t)

	org, account := newOrgWithAccount(t, app, "drifted", "Drifted", nil)

	// Somebody clears the account by hand. The organization still says active.
	account.Set("active", false)
	if err := app.Save(account); err != nil {
		t.Fatalf("hand-edit the account: %v", err)
	}

	// An ordinary organization edit that does not touch `active` at all.
	org.Set("description", "unrelated edit")
	if err := app.Save(org); err != nil {
		t.Fatalf("save the organization: %v", err)
	}

	if !accountActive(t, app, account.Id) {
		t.Error("an organization save did not pull its NATS account back into line; " +
			"the mirror is watching the edge instead of the value")
	}
}

// The platform's own organizations are refused, and an ordinary one is not.
// Paired, because a refusal test alone passes against a hook that refuses
// everything.
func TestInfrastructureOrgsCannotBeDeactivated(t *testing.T) {
	app := testutil.SetupApp(t)

	for _, flag := range []string{"is_operator_org", "is_system_org"} {
		t.Run(flag, func(t *testing.T) {
			org := newFlagOrg(t, app, flag, "Infra "+flag, map[string]any{flag: true})

			org.Set("active", false)
			err := app.Save(org)
			if err == nil {
				t.Fatalf("clearing active on an %s organization was allowed", flag)
			}
			if !errors.Is(err, hooks.ErrInfrastructureOrgActive) {
				t.Errorf("the refusal should be ErrInfrastructureOrgActive, got: %v", err)
			}

			// The flag really did not move, so the refusal came before the write.
			reloaded, findErr := app.FindRecordById("organizations", org.Id)
			if findErr != nil {
				t.Fatalf("reload the organization: %v", findErr)
			}
			if !reloaded.GetBool("active") {
				t.Error("the save was refused but the flag was written anyway")
			}
		})
	}

	// The pairing: an ordinary organization is still deactivatable.
	t.Run("an ordinary org still can be", func(t *testing.T) {
		org, account := newOrgWithAccount(t, app, "ordinary", "Ordinary Tenant", nil)
		org.Set("active", false)
		if err := app.Save(org); err != nil {
			t.Fatalf("an ordinary organization should be deactivatable: %v", err)
		}
		if accountActive(t, app, account.Id) {
			t.Error("the ordinary organization was deactivated but its account stayed active")
		}
	})
}

// A delete blanks the relations into an organization rather than removing them,
// so it needs the organization suspended (account withdrawn) and empty. Each
// refusal is paired with the same delete succeeding once its reason is gone.
func TestOrgDeleteNeedsItSuspendedAndEmpty(t *testing.T) {
	app := testutil.SetupApp(t)

	org, account := newOrgWithAccount(t, app, "doomed", "Doomed Tenant", nil)

	if err := app.Delete(org); !errors.Is(err, hooks.ErrOrgNotDeletable) {
		t.Fatalf("deleting an active organization: err = %v, want ErrOrgNotDeletable", err)
	}

	org.Set("active", false)
	if err := app.Save(org); err != nil {
		t.Fatalf("suspend the organization: %v", err)
	}

	locCol, err := app.FindCollectionByNameOrId("locations")
	if err != nil {
		t.Fatalf("locations collection: %v", err)
	}
	loc := core.NewRecord(locCol)
	loc.Set("name", "Last Site")
	loc.Set("organization", org.Id)
	if err := app.Save(loc); err != nil {
		t.Fatalf("create location: %v", err)
	}

	org, err = app.FindRecordById("organizations", org.Id)
	if err != nil {
		t.Fatalf("reload organization: %v", err)
	}
	if err := app.Delete(org); !errors.Is(err, hooks.ErrOrgNotDeletable) {
		t.Fatalf("deleting a suspended organization that holds a location: err = %v, want ErrOrgNotDeletable", err)
	}

	if err := app.Delete(loc); err != nil {
		t.Fatalf("delete location: %v", err)
	}
	if err := app.Delete(org); err != nil {
		t.Fatalf("deleting a suspended, empty organization should succeed: %v", err)
	}
	if accountActive(t, app, account.Id) {
		t.Error("the deleted organization's NATS account is active again: pb-nats would publish a tenant nobody owns")
	}
}
